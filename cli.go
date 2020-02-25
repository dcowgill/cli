/*

Package cli provides a minimalist template for building subcommand-based command
line programs. It depends only upon the Go standard library and emulates the
design of the 'go' program's interface.

Example progam:

	package main

	import (
		"fmt"

		"github.com/dcowgill/cli"
	)

	func main() {
		program := cli.MainProgram{
			Name:  "greeter",
			Short: "Example Program",
			Commands: []*cli.Command{
				cmdHello,
				helpTopic,
			},
		}
		program.Run()
	}

	var cmdHello = &cli.Command{
		Run:       runHello,
		UsageLine: "hello [-bye] name",
		Short:     "print a greeting to stdout",
		Long: `
	Prints a friendly greeting to the user!

	The -bye flag can be used to say goodbye instead of hello.`,
	}

	var helloFlags struct {
		bye bool
	}

	func init() {
		cmdHello.Flag.BoolVar(&helloFlags.bye, "bye", false, "say goodbye")
	}

	func runHello(cmd *cli.Command, args []string) {
		// The caller must provide a name to greet.
		if len(args) != 1 || args[0] == "" {
			cmd.Usage()
		}
		greeting := "Hello"
		if helloFlags.bye {
			greeting = "Goodbye"
		}
		fmt.Printf(greeting+", %s!\n", args[0])
	}

	var helpTopic = &cli.Command{
		Short: "help about a specific topic",
		Long:  `This is the help text.`,
	}

*/
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// A Command is an implementation of a command.
type Command struct {
	// Runs the command. The args are the unparsed (non flag)
	// arguments after the command name.
	//
	// If Run is nil, the Command is treated as a documentation
	// topic, not as a runnable command. This affects how it is
	// displayed in help and usage instructions.
	Run func(cmd *Command, args []string)

	// The one-line usage message. The first word in the line is
	// taken to be the command name.
	UsageLine string

	// The short description shown in the 'help' output.
	Short string

	// Long message shown in the 'help <command>' output. Will be
	// automatically trimmed of leading and trailing whitespace.
	Long string

	// The set of flags specific to this command.
	Flag flag.FlagSet
}

// Usage prints usage instructions to stderr, then exits with an error
// code. Call this method from within the command's Run function to
// signal that its command line arguments are invalid.
func (c *Command) Usage() {
	c.printUsage(os.Stderr)
	os.Exit(2)
}

// Reports the command's name: the first word in the usage line.
func (c *Command) name() string {
	name := c.UsageLine
	i := strings.Index(name, " ")
	if i >= 0 {
		name = name[:i]
	}
	return name
}

// Returns true if the command has any flags.
func (c *Command) hasFlags() bool {
	numFlags := 0
	c.Flag.VisitAll(func(*flag.Flag) { numFlags++ })
	return numFlags != 0
}

// Prints detailed usage instructions to the writer.
func (c *Command) printUsage(w io.Writer) {
	if c.runnable() {
		fmt.Fprintf(w, "usage: %s\n\n", c.UsageLine)
		if c.hasFlags() {
			c.Flag.PrintDefaults()
			fmt.Fprint(w, "\n")
		}
	}
	fmt.Fprintf(w, "%s\n", strings.TrimSpace(c.Long))
}

// Reports whether the command can be run; otherwise it is a
// documentation pseudo-command.
func (c *Command) runnable() bool {
	return c.Run != nil
}

// MainProgram defines a command line program whose interface is based
// on subcommands. Each subcommand has its own private set of flags.
type MainProgram struct {
	// The name of the program's executable file. It is included in
	// usage instructions so it should match the filename exactly.
	Name string

	// The short description shown in the 'help' output.
	Short string

	// Lists the available commands and help topics. The order here
	// is the order in which they are printed by the help command.
	Commands []*Command
}

// Run executes the program. It does not return.
func (mp *MainProgram) Run() {
	// Parse the command line.
	flag.Usage = mp.usageError
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		mp.usageError()
	}

	// Handle the help command.
	if args[0] == "help" {
		mp.help(args[1:])
		os.Exit(0)
	}

	// Find the command and run it.
	for _, cmd := range mp.Commands {
		if cmd.name() == args[0] && cmd.runnable() {
			cmd.Flag.Usage = func() { cmd.Usage() }
			cmd.Flag.Parse(args[1:])
			cmd.Run(cmd, cmd.Flag.Args())
			os.Exit(0)
		}
	}

	// The user specified an unknown command.
	fmt.Fprintf(os.Stderr, "%s: unknown command %q\n", mp.Name, args[0])
	fmt.Fprintf(os.Stderr, "Run '%s help' for usage.\n", mp.Name)
	os.Exit(2)
}

// Implements the help command.
func (mp *MainProgram) help(args []string) {
	if len(args) == 0 {
		mp.printUsage(os.Stdout)
		os.Exit(0)
	}
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s help command\n", mp.Name)
		fmt.Fprintf(os.Stderr, "Too many arguments given.\n")
		os.Exit(2)
	}

	arg := args[0]
	for _, cmd := range mp.Commands {
		if cmd.name() == arg {
			cmd.printUsage(os.Stdout)
			os.Exit(0)
		}
	}

	fmt.Fprintf(os.Stderr, "Unknown help topic %q. Run '%s help'.\n", arg, mp.Name)
	os.Exit(2)
}

// Prints usage instructions to stderr, then calls exit(2).
func (mp *MainProgram) usageError() {
	mp.printUsage(os.Stderr)
	os.Exit(2)
}

// Prints detailed usage instructions to the writer.
func (mp *MainProgram) printUsage(w io.Writer) {
	// Format the commands and help topics.
	nameWidth := 15
	for _, cmd := range mp.Commands {
		if w := len(cmd.name()) + 2; w > nameWidth {
			nameWidth = w
		}
	}
	format := fmt.Sprintf("\t%%-%ds%%s\n", nameWidth)
	var cmdList, docList string
	for _, cmd := range mp.Commands {
		s := fmt.Sprintf(format, cmd.name(), cmd.Short)
		if cmd.runnable() {
			cmdList += s
		} else {
			docList += s
		}
	}

	// Generate the help instructions.
	usage := fmt.Sprintf(`%[2]s

Usage:

	%[1]s command [arguments]

The commands are:

%[3]s
Use "%[1]s help [command]" for more information about a command.

`, mp.Name, mp.Short, cmdList)

	if docList != "" {
		usage += fmt.Sprintf(`Additional help topics:

%[2]s
Use "%[1]s help [topic]" for more information about that topic.

`, mp.Name, docList)
	}

	// Print.
	fmt.Fprint(w, usage)
}
