package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	// 3rd Party Pacakges
	"github.com/robertkrimen/otto"

	// Caltech Library Pacakges
	"github.com/caltechlibrary/ostdlib"
)

var (
	showHelp    bool
	showVersion bool
	runRepl     bool
)

func check(expr bool, msg string, err error) {
	if expr == true {
		log.Fatalf("%s, %s", msg, err)
	}
}

func init() {
	flag.BoolVar(&showHelp, "h", false, "display this help information")
	flag.BoolVar(&showVersion, "v", false, "display version information")
	flag.BoolVar(&runRepl, "i", false, "Run in interactive mode")
}

func main() {
	flag.Parse()

	// Process command line switches
	switch {
	case showHelp == true:
		fmt.Println(`
 USAGE: ottomatic [OPTIONS] [JAVASCRIPT_FILENAMES]

`)
		// FIXME: this writes to stderr, need to write to stdout
		flag.PrintDefaults()
		fmt.Printf("\nVersion %s\n", ostdlib.Version)
		os.Exit(0)
	case showVersion == true:
		fmt.Printf("\nVersion %s\n", ostdlib.Version)
		os.Exit(0)
	}

	// Create our JavaScriptVM
	vm := otto.New()
	js := ostdlib.New(vm)

	// Add objects (e.g. os, http and polyfills)
	js.AddExtensions()
	// Add extension help
	js.AddHelp()
	// Add autocomplete based on current state of js.Help
	js.AddAutoComplete()

	args := flag.Args()
	if len(args) == 0 {
		runRepl = true
	}
	for _, scriptname := range args {
		src, err := ioutil.ReadFile(scriptname)
		check((err != nil), scriptname, err)

		script, err := vm.Compile(scriptname, src)
		check((err != nil), scriptname, err)

		_, err = vm.Eval(script)
		check((err != nil), scriptname, err)
	}

	if runRepl == true {
		js.Repl()
	}
	os.Exit(0)
}
