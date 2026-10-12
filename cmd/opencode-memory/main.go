// Command opencode-memory stores memories as markdown files and searches them.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/zarathos364/opencode-memory/internal/indexer"
	"github.com/zarathos364/opencode-memory/internal/service"
	"github.com/zarathos364/opencode-memory/internal/storage"
)

const usageText = `Usage: opencode-memory COMMAND [OPTION]... [ARG]...

Store memories as markdown files and keep a full-text index over them.

Commands:
  add       store a new memory
  search    find memories by their text
  list      list stored memories, oldest first
  get       print one memory
  update    change part of a memory
  delete    remove one memory
  reindex   rebuild the index from the stored files

Every command accepts --root DIR to pick the store; otherwise the closest
.opencode directory above the working directory is used.

Run 'opencode-memory COMMAND --help' to see that command's options.

      --version    output version information and exit
  -h, --help       display this help and exit
`

// errHelp stands for the help requested through a command flag.
var errHelp = errors.New("help requested")

// Replaced at build time through the linker.
var version = "dev"

type memoryJSON struct {
	ID        string   `json:"id"`
	CreatedAt string   `json:"created_at"`
	Title     string   `json:"title"`
	Tags      []string `json:"tags"`
	Sensitive bool     `json:"sensitive"`
	Content   string   `json:"content"`
	Score     float64  `json:"score,omitempty"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "opencode-memory:", err)
		os.Exit(1)
	}
}

func run(args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		if err := printUsage(errOut); err != nil {
			return err
		}
		return errors.New("no command given")
	}

	command, rest := args[0], args[1:]
	var err error
	switch command {
	case "add":
		err = runAdd(rest, out, errOut)
	case "search":
		err = runSearch(rest, out, errOut)
	case "list":
		err = runList(rest, out, errOut)
	case "get":
		err = runGet(rest, out, errOut)
	case "update":
		err = runUpdate(rest, out, errOut)
	case "delete":
		err = runDelete(rest, out, errOut)
	case "reindex":
		err = runReindex(rest, out, errOut)
	case "--version":
		return printLine(out, "opencode-memory %s\n", version)
	case "help", "-h", "--help":
		return printUsage(out)
	default:
		if usageErr := printUsage(errOut); usageErr != nil {
			return usageErr
		}
		return fmt.Errorf("unknown command %q", command)
	}

	if errors.Is(err, errHelp) {
		return nil
	}
	return err
}

func runAdd(args []string, out, errOut io.Writer) error {
	flags := newFlagSet("add", errOut)
	root := flags.String("root", "", "store location")
	title := flags.String("title", "", "short title, required")
	tags := flags.String("tags", "", "comma separated tags")
	sensitive := flags.Bool("sensitive", false, "store without making it searchable")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if *title == "" {
		return errors.New("add needs --title")
	}
	if flags.NArg() == 0 {
		return errors.New("add needs the memory text")
	}

	return withService(*root, func(svc *service.MemoryService) error {
		created, err := svc.Create(*title, strings.Join(flags.Args(), " "), splitTags(*tags), *sensitive)
		if err != nil {
			return err
		}
		return printLine(out, "%s\n", created.ID)
	})
}

func runSearch(args []string, out, errOut io.Writer) error {
	flags := newFlagSet("search", errOut)
	root := flags.String("root", "", "store location")
	limit := flags.Int("limit", indexer.DefaultLimit, "maximum number of results")
	asJSON := flags.Bool("json", false, "print results as JSON")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() == 0 {
		return errors.New("search needs a query")
	}

	return withService(*root, func(svc *service.MemoryService) error {
		if err := warnIfOutOfSync(svc, errOut); err != nil {
			return err
		}

		results, err := svc.Search(strings.Join(flags.Args(), " "), *limit)
		if err != nil {
			return err
		}
		if *asJSON {
			return writeJSON(out, toSearchJSON(results))
		}
		for _, result := range results {
			err := printLine(out, "score=%.4f  id=%s  %s\n", result.Score, result.Memory.ID, result.Memory.Title)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func runList(args []string, out, errOut io.Writer) error {
	flags := newFlagSet("list", errOut)
	root := flags.String("root", "", "store location")
	showSensitive := flags.Bool("sensitive", false, "include memories kept out of search")
	asJSON := flags.Bool("json", false, "print memories as JSON")
	if err := parseFlags(flags, args); err != nil {
		return err
	}

	return withService(*root, func(svc *service.MemoryService) error {
		if err := warnIfOutOfSync(svc, errOut); err != nil {
			return err
		}

		memories, err := svc.List()
		if err != nil {
			return err
		}

		visible := make([]*storage.Memory, 0, len(memories))
		for _, m := range memories {
			if m.Sensitive && !*showSensitive {
				continue
			}
			visible = append(visible, m)
		}

		if *asJSON {
			return writeJSON(out, toMemoryJSON(visible))
		}
		for _, m := range visible {
			err := printLine(out, "%s  id=%s  %s\n", m.CreatedAt.Format(time.RFC3339), m.ID, m.Title)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func runGet(args []string, out, errOut io.Writer) error {
	flags := newFlagSet("get", errOut)
	root := flags.String("root", "", "store location")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("get needs exactly one id")
	}

	return withService(*root, func(svc *service.MemoryService) error {
		m, err := svc.Get(flags.Arg(0))
		if err != nil {
			return err
		}
		return printLine(out, "id: %s\ntitle: %s\ncreated: %s\ntags: %s\nsensitive: %t\n\n%s\n",
			m.ID, m.Title, m.CreatedAt.Format(time.RFC3339), strings.Join(m.Tags, ", "), m.Sensitive, m.Content)
	})
}

func runUpdate(args []string, out, errOut io.Writer) error {
	flags := newFlagSet("update", errOut)
	root := flags.String("root", "", "store location")
	title := flags.String("title", "", "replace the title")
	tags := flags.String("tags", "", "replace the tags")
	sensitive := flags.Bool("sensitive", false, "replace the sensitive flag")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() == 0 {
		return errors.New("update needs an id")
	}

	set := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { set[f.Name] = true })

	patch := service.Patch{}
	if set["title"] {
		patch.Title = title
	}
	if set["tags"] {
		replacement := splitTags(*tags)
		patch.Tags = &replacement
	}
	if set["sensitive"] {
		patch.Sensitive = sensitive
	}
	if flags.NArg() > 1 {
		replacement := strings.Join(flags.Args()[1:], " ")
		patch.Content = &replacement
	}
	if patch.Title == nil && patch.Content == nil && patch.Tags == nil && patch.Sensitive == nil {
		return errors.New("update needs --title, --tags, --sensitive or new text")
	}

	return withService(*root, func(svc *service.MemoryService) error {
		updated, err := svc.Update(flags.Arg(0), patch)
		if err != nil {
			return err
		}
		return printLine(out, "%s\n", updated.ID)
	})
}

func runDelete(args []string, out, errOut io.Writer) error {
	flags := newFlagSet("delete", errOut)
	root := flags.String("root", "", "store location")
	if err := parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("delete needs exactly one id")
	}

	return withService(*root, func(svc *service.MemoryService) error {
		if err := svc.Delete(flags.Arg(0)); err != nil {
			return err
		}
		return printLine(out, "deleted %s\n", flags.Arg(0))
	})
}

func runReindex(args []string, out, errOut io.Writer) error {
	flags := newFlagSet("reindex", errOut)
	root := flags.String("root", "", "store location")
	if err := parseFlags(flags, args); err != nil {
		return err
	}

	return withService(*root, func(svc *service.MemoryService) error {
		stored, err := svc.Reindex()
		if err != nil {
			return err
		}
		return printLine(out, "reindexed %d memories\n", stored)
	})
}

func newFlagSet(name string, errOut io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(errOut)
	return flags
}

func parseFlags(flags *flag.FlagSet, args []string) error {
	if err := flags.Parse(reorderFlags(flags, args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return errHelp
		}
		return err
	}
	return nil
}

// The flag package stops parsing at the first argument that does not start with
// a dash, so a flag written after an id or a query would be read as part of it.
// Every flag, together with the value it consumes, is therefore moved in front of
// the positional arguments before parsing.
func reorderFlags(flags *flag.FlagSet, args []string) []string {
	options := make([]string, 0, len(args))
	operands := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			options = append(options, arg)
			operands = append(operands, args[i+1:]...)
			break
		}

		name, hasValue := flagName(arg)
		if name == "" {
			operands = append(operands, arg)
			continue
		}

		options = append(options, arg)
		definition := flags.Lookup(name)
		// An unknown name is left where it is so the flag package reports it, and
		// a boolean flag never takes the argument that follows it.
		if definition == nil || hasValue || !consumesValue(definition) {
			continue
		}
		if i+1 < len(args) {
			i++
			options = append(options, args[i])
		}
	}
	return append(options, operands...)
}

func flagName(arg string) (string, bool) {
	if len(arg) < 2 || arg[0] != '-' {
		return "", false
	}
	name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
	return name, hasValue
}

func consumesValue(definition *flag.Flag) bool {
	boolean, ok := definition.Value.(interface{ IsBoolFlag() bool })
	return !ok || !boolean.IsBoolFlag()
}

// withService resolves the store, opens it, hands it to work and closes it
// afterwards, so every command holds the index the same way.
func withService(root string, work func(*service.MemoryService) error) error {
	svc, err := openService(root)
	if err != nil {
		return err
	}
	defer func() { _ = svc.Close() }()
	return work(svc)
}

// openService resolves the store location and opens it, turning the two failures
// that need an explanation into a message a caller can act on.
func openService(root string) (*service.MemoryService, error) {
	start, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("read working directory: %w", err)
	}
	resolved, err := storage.ResolveRoot(root, start)
	if err != nil {
		return nil, err
	}

	svc, err := service.New(resolved)
	if err != nil {
		if indexer.IsLockError(err) {
			return nil, fmt.Errorf("%w: another instance is using this store", err)
		}
		return nil, indexFailure(resolved, err)
	}
	return svc, nil
}

// indexFailure says where the index lives and which step failed. Removing the
// directory is offered only when there is one: it cannot fix a path that could
// not be created in the first place.
func indexFailure(root string, err error) error {
	dir := storage.IndexDir(root)
	if _, statErr := os.Stat(dir); statErr != nil {
		return fmt.Errorf("create the search index in %s: %w", dir, err)
	}
	return fmt.Errorf("open the search index in %s: %w (remove that directory to rebuild it)", dir, err)
}

// warnIfOutOfSync reports a drifted index without failing the command, because the
// stored files are still readable and can be reindexed at any moment.
func warnIfOutOfSync(svc *service.MemoryService, errOut io.Writer) error {
	consistent, err := svc.Consistent()
	if err != nil {
		return nil
	}
	if consistent {
		return nil
	}
	_, err = fmt.Fprintln(errOut, "warning: the search index is out of sync, run 'opencode-memory reindex'")
	if err != nil {
		return fmt.Errorf("write warning: %w", err)
	}
	return nil
}

func printUsage(w io.Writer) error {
	if _, err := fmt.Fprint(w, usageText); err != nil {
		return fmt.Errorf("write usage: %w", err)
	}
	return nil
}

func printLine(w io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func splitTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

func newMemoryJSON(m *storage.Memory, score float64) memoryJSON {
	return memoryJSON{
		ID:        m.ID,
		CreatedAt: m.CreatedAt.Format(time.RFC3339Nano),
		Title:     m.Title,
		Tags:      m.Tags,
		Sensitive: m.Sensitive,
		Content:   m.Content,
		Score:     score,
	}
}

func toMemoryJSON(memories []*storage.Memory) []memoryJSON {
	out := make([]memoryJSON, 0, len(memories))
	for _, m := range memories {
		out = append(out, newMemoryJSON(m, 0))
	}
	return out
}

func toSearchJSON(results []service.Result) []memoryJSON {
	out := make([]memoryJSON, 0, len(results))
	for _, result := range results {
		out = append(out, newMemoryJSON(result.Memory, result.Score))
	}
	return out
}

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}
	return nil
}
