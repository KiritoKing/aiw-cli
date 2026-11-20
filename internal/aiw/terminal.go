package aiw

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"golang.org/x/term"
)

func terminalFD(stream any) (int, bool) {
	f, ok := stream.(*os.File)
	if !ok || f == nil {
		return 0, false
	}
	fd := int(f.Fd())
	return fd, term.IsTerminal(fd)
}

func (r commandEnv) isInteractive() bool {
	_, ci := os.LookupEnv("CI")
	_, in := terminalFD(r.In)
	_, out := terminalFD(r.Out)
	_, errOut := terminalFD(r.ErrOut)
	return in && out && errOut && !r.noInput && !r.jsonOutput && !ci
}

func (r commandEnv) colorEnabled() bool {
	_, noColor := os.LookupEnv("NO_COLOR")
	_, tty := terminalFD(r.Out)
	return tty && !r.noColor && !r.jsonOutput && !noColor && os.Getenv("TERM") != "dumb"
}

func terminalLabel(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
}

func (r commandEnv) printTable(headers []string, rows [][]string, statusColumns ...int) error {
	renderer := lipgloss.NewRenderer(r.Out)
	color := r.colorEnabled()
	for _, row := range rows {
		for col, cell := range row {
			row[col] = terminalLabel(cell)
		}
	}
	t := table.New().Headers(headers...).Rows(rows...).Wrap(true).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderHeader(false).BorderColumn(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := renderer.NewStyle().PaddingRight(2)
			if !color {
				return style
			}
			if row == table.HeaderRow {
				return style.Bold(true)
			}
			for _, statusCol := range statusColumns {
				if col == statusCol {
					return style.Foreground(lipgloss.Color(statusColor(rows[row][col])))
				}
			}
			return style
		})
	if fd, ok := terminalFD(r.Out); ok {
		if width, _, err := term.GetSize(fd); err == nil && width >= 40 {
			// Only constrain wide tables, leaving short ones at natural width.
			rendered := t.Render()
			if lipgloss.Width(rendered) > width {
				t.Width(width)
			} else {
				_, err := fmt.Fprintln(r.Out, rendered)
				return err
			}
		}
	}
	_, err := fmt.Fprintln(r.Out, t.Render())
	return err
}

func statusColor(status string) string {
	switch {
	case status == "pass" || status == "success" || status == "registered" || status == "clean":
		return "2"
	case status == "fail" || status == "failed" || status == "drift":
		return "1"
	case status == "warn" || status == "running" || status == "stale" || strings.HasPrefix(status, "dirty"):
		return "3"
	default:
		return "7"
	}
}

func (r commandEnv) withProgress(title string, action func(commandEnv) error) error {
	if !r.isInteractive() {
		return action(r)
	}
	if r.verbose != nil || os.Getenv("TERM") == "dumb" {
		fmt.Fprintln(r.ErrOut, title)
		return action(r)
	}
	var output strings.Builder
	work := r
	work.Out = &output
	ctx, done := context.WithCancel(r.ctx)
	style := lipgloss.NewRenderer(r.ErrOut).NewStyle()
	if r.colorEnabled() {
		style = style.Foreground(lipgloss.Color("6"))
	}
	indicator := spinner.New().Title(title).Style(style).TitleStyle(style)
	program := tea.NewProgram(indicator, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(r.ErrOut), tea.WithoutSignalHandler())
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_, _ = program.Run()
	}()
	err := action(work)
	done()
	<-finished
	_, writeErr := io.WriteString(r.Out, output.String())
	return errors.Join(err, writeErr)
}

func (r commandEnv) runForm(field huh.Field) error {
	form := huh.NewForm(huh.NewGroup(field)).WithInput(r.In).WithOutput(r.ErrOut).
		WithProgramOptions(tea.WithoutSignalHandler())
	if !r.colorEnabled() {
		theme := huh.ThemeBase()
		theme.Focused.TextInput.Placeholder = lipgloss.NewStyle()
		theme.Blurred.TextInput.Placeholder = lipgloss.NewStyle()
		theme.Help.ShortKey = lipgloss.NewStyle()
		theme.Help.ShortDesc = lipgloss.NewStyle()
		theme.Help.ShortSeparator = lipgloss.NewStyle()
		form.WithTheme(theme)
	}
	return form.RunWithContext(r.ctx)
}

func (c ChangeContext) chooseRepos() ([]string, error) {
	if len(c.Config.Repos) == 0 {
		return nil, fmt.Errorf("aiw.yaml declares no repos")
	}
	var names []string
	if _, err := os.Stat(specPath(c.Root)); err == nil {
		spec, err := readSpec(c.Root)
		if err != nil {
			return nil, err
		}
		for _, name := range spec.Repos {
			if _, ok := c.Config.Repos[name]; ok {
				names = append(names, name)
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	options := make([]huh.Option[string], 0, len(c.Config.Repos))
	for _, name := range repoNames(c.Config) {
		repo := c.Config.Repos[name]
		label := name + " (" + repo.Base + ")"
		if description := repoDescription(repo.Metadata); description != "" {
			label += " · " + terminalLabel(description)
		}
		options = append(options, huh.NewOption(label, name))
	}
	field := huh.NewMultiSelect[string]().Title("Select repositories for " + c.Branch).
		Options(options...).Value(&names).Validate(func(selection []string) error {
		if len(selection) == 0 {
			return fmt.Errorf("select at least one repository")
		}
		return nil
	})
	if err := c.runForm(field); err != nil {
		return nil, err
	}
	return names, nil
}
