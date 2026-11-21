package aiw

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func Run(args []string) error {
	return RunContext(context.Background(), args)
}

func RunContext(ctx context.Context, args []string) error {
	return RunWithIO(ctx, args, defaultIO())
}

func RunWithIO(ctx context.Context, args []string, streams IO) error {
	r := newCommandEnv(ctx, streams)
	root := newRootCommand(r)
	// Cobra uses the process argv when SetArgs receives nil.
	if args == nil {
		args = []string{}
	}
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

type commandOptions struct {
	cwd     string
	verbose bool
	noInput bool
	noColor bool
}

func newRootCommand(env commandEnv) *cobra.Command {
	var options commandOptions
	root := &cobra.Command{
		Use:           "aiw",
		Short:         "Assemble business Git worktrees in a control-plane-created Agent Repo worktree",
		Version:       buildVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE:          func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.SetIn(env.In)
	root.SetOut(env.Out)
	root.SetErr(env.ErrOut)
	flags := root.PersistentFlags()
	flags.StringVarP(&options.cwd, "cwd", "C", "", "run from this directory")
	flags.BoolVar(&options.noColor, "no-color", false, "disable terminal colors")
	flags.BoolVar(&options.noInput, "no-input", false, "disable interactive prompts")
	flags.BoolVar(&options.verbose, "verbose", false, "log Git commands and execution times to stderr")
	_ = root.MarkPersistentFlagDirname("cwd")

	runEnv := func(cmd *cobra.Command) (commandEnv, error) {
		r := env
		r.noColor, r.noInput = options.noColor, options.noInput
		r.jsonOutput, _ = cmd.Flags().GetBool("json")
		if options.verbose {
			r.verbose = r.ErrOut
		}
		return r.inDirectory(options.cwd)
	}
	bind := func(action func(commandEnv, []string) error) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			r, err := runEnv(cmd)
			if err != nil {
				return err
			}
			return action(r, args)
		}
	}

	var skipSkills bool
	initCmd := leaf("init <agent-repo-path>", "Initialize an Agent Repo", cobra.ExactArgs(1), bind(func(r commandEnv, args []string) error {
		return r.initProject(args[0], skipSkills)
	}))
	initCmd.ValidArgsFunction = completeDirectory
	initCmd.Flags().BoolVar(&skipSkills, "skip-skills", false, "skip installing the default skills")
	root.AddCommand(initCmd)
	root.AddCommand(leaf("version", "Print the AIW version", cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
		cmd.Println(buildVersion())
		return nil
	}))
	doctor := leaf("doctor", "Check tools and the current Agent Repo", cobra.NoArgs, bind(func(r commandEnv, _ []string) error {
		return r.doctor()
	}))
	jsonFlag(doctor)
	root.AddCommand(doctor)
	agents := commandGroup("agents", "Manage Agent Repo instructions")
	agents.AddCommand(leaf("sync", "Update the managed AGENTS.md block", cobra.NoArgs, bind(func(r commandEnv, _ []string) error {
		return r.syncAgents()
	})))
	root.AddCommand(agents)
	workspace := commandGroup("workspace", "Manage the generated VS Code workspace")
	workspace.AddCommand(leaf("sync", "Regenerate aiw.code-workspace", cobra.NoArgs, bind(func(r commandEnv, _ []string) error {
		return r.syncWorkspace()
	})))
	root.AddCommand(workspace)

	repo := commandGroup("repo", "Register, list and sync business repositories")
	bindRepo := func(action func(commandEnv, string, string, []string) error, lock bool) func(*cobra.Command, []string) error {
		return bind(func(r commandEnv, args []string) error {
			root, err := projectRoot(r.Cwd)
			if err != nil {
				return err
			}
			home, err := r.dataHome()
			if err != nil {
				return err
			}
			if lock {
				return r.withLock(filepath.Join(home, "registry.lock"), func() error {
					return action(r, root, home, args)
				})
			}
			return action(r, root, home, args)
		})
	}
	list := leaf("list", "List configured business repositories", cobra.NoArgs, bindRepo(func(r commandEnv, root, home string, _ []string) error {
		return r.repoList(root, home)
	}, false))
	jsonFlag(list)
	var source string
	register := leaf("register <name>", "Register an existing local checkout", func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}
		if source == "" {
			return fmt.Errorf("--source is required")
		}
		return nil
	}, bindRepo(func(r commandEnv, root, home string, args []string) error {
		return r.withProgress("Registering repository…", func(work commandEnv) error {
			return work.registerRepo(root, home, args[0], source)
		})
	}, true))
	register.Flags().StringVar(&source, "source", "", "existing local checkout")
	_ = register.MarkFlagDirname("source")
	scan := leaf("scan <dir>", "Find and register matching local checkouts", cobra.ExactArgs(1), bindRepo(func(r commandEnv, root, home string, args []string) error {
		return r.withProgress("Scanning for repositories…", func(work commandEnv) error {
			return work.scanRepos(root, home, args[0])
		})
	}, true))
	scan.ValidArgsFunction = completeDirectory
	sync := leaf("sync [name]", "Fetch configured baselines into the managed stores", cobra.MaximumNArgs(1), bindRepo(func(r commandEnv, root, home string, args []string) error {
		return r.withProgress("Syncing repositories…", func(work commandEnv) error {
			return work.syncRepo(root, home, firstArg(args))
		})
	}, true))
	repo.AddCommand(list, register, scan, sync)
	root.AddCommand(repo)

	change := commandGroup("change", "Manage business worktrees for a Change")
	change.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		if os.Getenv("AIW_LIFECYCLE_ACTIVE") == "1" {
			return fmt.Errorf("AIW Change commands cannot run inside a repository setup or cleanup script")
		}
		return nil
	}
	bindChange := func(action func(ChangeContext, []string) error) func(*cobra.Command, []string) error {
		return bind(func(r commandEnv, args []string) error {
			c, err := r.currentChange()
			if err != nil {
				return err
			}
			return c.withChangeLock(func() error { return action(c, args) })
		})
	}

	var names []string
	selectCmd := leaf("select", "Select business repositories for this Change", func(cmd *cobra.Command, args []string) error {
		if err := cobra.NoArgs(cmd, args); err != nil {
			return err
		}
		promptEnv := env
		promptEnv.noInput = options.noInput
		if len(names) == 0 && !promptEnv.isInteractive() {
			return fmt.Errorf("usage: aiw change select --repo <name> [--repo <name>...]")
		}
		return nil
	}, bindChange(func(c ChangeContext, _ []string) error {
		selected := names
		if len(selected) == 0 {
			var err error
			selected, err = c.chooseRepos()
			if err != nil {
				return err
			}
		}
		return c.selectRepos(selected)
	}))
	selectCmd.Flags().StringArrayVar(&names, "repo", nil, "repository to select (repeatable)")
	add := leaf("add-repo <name>", "Add a repository to the Change selection", cobra.ExactArgs(1), bindChange(func(c ChangeContext, args []string) error {
		return c.updateSelection("add", args[0])
	}))
	remove := leaf("remove-repo <name>", "Remove a repository from the Change selection", cobra.ExactArgs(1), bindChange(func(c ChangeContext, args []string) error {
		return c.updateSelection("remove", args[0])
	}))
	materialize := leaf("materialize", "Reconcile business worktrees and run setup", cobra.NoArgs, bindChange(func(c ChangeContext, _ []string) error {
		return c.materialize(false)
	}))
	repair := leaf("repair", "Recover worktrees and links after interruption", cobra.NoArgs, bindChange(func(c ChangeContext, _ []string) error {
		return c.repair()
	}))
	status := leaf("status", "Show Git status and readiness", cobra.NoArgs, bindChange(func(c ChangeContext, _ []string) error {
		return c.status()
	}))
	jsonFlag(status)
	setup := leaf("setup [name]", "Run pending setup or retry an explicitly named repository", cobra.MaximumNArgs(1), bindChange(func(c ChangeContext, args []string) error {
		return c.setup(args)
	}))
	cleanup := leaf("cleanup <name>", "Run cleanup for a materialized repository", cobra.ExactArgs(1), bindChange(func(c ChangeContext, args []string) error {
		return c.cleanup(args[0])
	}))
	var closeOptions CloseOptions
	closeCmd := leaf("close", "Run cleanup and release AIW business worktrees", cobra.NoArgs, bindChange(func(c ChangeContext, _ []string) error {
		return c.close(closeOptions)
	}))
	closeCmd.Flags().BoolVar(&closeOptions.AllowUnpushed, "allow-unpushed", false, "close when remote branch equality is unverified")
	closeCmd.Flags().BoolVar(&closeOptions.Discard, "discard-changes", false, "discard local changes in AIW business worktrees")
	changeList := leaf("list", "List registered Changes for the current project", cobra.NoArgs, bind(func(r commandEnv, _ []string) error {
		return r.changeList()
	}))
	jsonFlag(changeList)
	lookup := bind(func(r commandEnv, args []string) error { return r.changePath(args[0]) })
	path := leaf("path <branch>", "Show the control-plane worktree path", cobra.ExactArgs(1), lookup)
	open := leaf("open <branch>", "Return the existing control-plane worktree path", cobra.ExactArgs(1), lookup)
	change.AddCommand(selectCmd, add, remove, materialize, repair, status, setup, cleanup, closeCmd, changeList, path, open)
	root.AddCommand(change)

	repoCompletion := completeRepos(runEnv)
	for _, cmd := range []*cobra.Command{register, sync, add, remove, setup, cleanup} {
		cmd.ValidArgsFunction = repoCompletion
	}
	_ = selectCmd.RegisterFlagCompletionFunc("repo", repoCompletion)
	path.ValidArgsFunction = completeBranches(runEnv)
	open.ValidArgsFunction = path.ValidArgsFunction
	root.InitDefaultCompletionCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "completion" {
			cmd.RunE = requireSubcommand
			break
		}
	}
	return root
}

func leaf(use, short string, args cobra.PositionalArgs, run func(*cobra.Command, []string) error) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Args: args, RunE: run}
}

func commandGroup(name, short string) *cobra.Command {
	return leaf(name, short, cobra.NoArgs, requireSubcommand)
}

func requireSubcommand(cmd *cobra.Command, _ []string) error {
	return fmt.Errorf("%s requires a subcommand; run %s --help", cmd.CommandPath(), cmd.CommandPath())
}

func jsonFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "output JSON")
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
