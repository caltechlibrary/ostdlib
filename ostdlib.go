//
// Package ostdlib is a collection of JavaScript objects, functions and polyfill for standardizing
// embedding Robert Krimen's Otto JavaScript Interpreter.
//
// @author R. S. Doiel, <rsdoiel@caltech.edu>
//
// Copyright (c) 2016, Caltech
// All rights not granted herein are expressly reserved by Caltech.
//
// Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.
//
// 3. Neither the name of the copyright holder nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
//
package ostdlib

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	// 3rd Party packages
	"github.com/chzyer/readline"
	"github.com/robertkrimen/otto"
)

// Version of the Otto Standard Library
const Version = "0.0.0"

// Polyfill addes missing functionality implemented in JavaScript rather than Go
var Polyfill = `
	if (!Date.prototype.now) {
		Date.prototype.now = function now() {
			'use strict';
		 	return new Date().getTime();
		};
	}
	if (!String.prototype.repeat) {
	  String.prototype.repeat = function(count) {
	    'use strict';
	    if (this == null) {
	      throw new TypeError('can\'t convert ' + this + ' to object');
	    }
	    var str = '' + this;
	    count = +count;
	    if (count != count) {
	      count = 0;
	    }
	    if (count < 0) {
	      throw new RangeError('repeat count must be non-negative');
	    }
	    if (count == Infinity) {
	      throw new RangeError('repeat count must be less than infinity');
	    }
	    count = Math.floor(count);
	    if (str.length == 0 || count == 0) {
	      return '';
	    }
	    // Ensuring count is a 31-bit integer allows us to heavily optimize the
	    // main part. But anyway, most current (August 2014) browsers can't handle
	    // strings 1 << 28 chars or longer, so:
	    if (str.length * count >= 1 << 28) {
	      throw new RangeError('repeat count must not overflow maximum string size');
	    }
	    var rpt = '';
	    for (;;) {
	      if ((count & 1) == 1) {
	        rpt += str;
	      }
	      count >>>= 1;
	      if (count == 0) {
	        break;
	      }
	      str += str;
	    }
	    // Could we try:
	    // return Array(count + 1).join(this);
	    return rpt;
	  }
	}
`

// JavaScriptVM is a wrapper for *otto.Otto to make it easy to add features without forking Otto.
// FIXME: Need to come up with a clean way to consistantly extend Otto's JavaScript VM
// and easily add helpful features like ToStruct(), additional AutoComplete phrases, smart help, etc.
// Help is structured accessed through a map of maps where k1 = Object, k2 = function name, value help message
type JavaScriptVM struct {
	VM            *otto.Otto
	AutoCompleter readline.AutoCompleter
	Help          map[string]map[string]*HelpMsg
}

type HelpMsg struct {
	Object   string
	Function string
	Params   []string
	Msg      string
}

// New create a new JavaScriptVM structure extending the functionality of *otto.Otto
func New(vm *otto.Otto) *JavaScriptVM {
	js := new(JavaScriptVM)
	js.VM = vm
	// FIXME: Look at SetChildren() and GetChildren to augument this auto complete list.
	js.AutoCompleter = readline.NewPrefixCompleter(
		// Autocomplete for os object
		readline.PcItem("os.args()"),
		readline.PcItem("os.exit(exitCode)"),
		readline.PcItem("os.getEnv(envvar)"),
		readline.PcItem("os.readFile(filename)"),
		readline.PcItem("os.writeFile(filename, data)"),
		readline.PcItem("os.rename(oldname, newname)"),
		readline.PcItem("os.remove(filename)"),
		readline.PcItem("os.chmod(filename, perms)"),
		readline.PcItem("os.find(filename)"),
		readline.PcItem("os.mkdir(dirname)"),
		readline.PcItem("os.mkdirAll(dirpath)"),
		readline.PcItem("os.rmdir(dirname)"),
		readline.PcItem("os.rmdirAll(dirpath)"),
		// Autocompleter for http object
		readline.PcItem("http.get(url, headers)"),
		readline.PcItem("http.post(url, headers, payload)"),
	)
	return js
}

// SetHelp adds help documentation by object and function
func (js *JavaScriptVM) SetHelp(objectName string, functionName string, params []string, text string) {
	help := new(HelpMsg)
	help.Object = objectName
	help.Function = functionName
	help.Params = params
	help.Msg = text
	//FIXME: How to I make sure js.Help is not a nil map?
	js.Help[objectName][functionName] = help
}

// GetHelp retrieves help text by object and function names
func (js *JavaScriptVM) GetHelp(objectName, functionName string) string {
	if objectName == "" {
		objectName = "-"
	}
	if functionName == "" {
		functionName = "-"
	}
	if msg, ok := js.Help[objectName][functionName]; ok == true {
		switch {
		case objectName == "-" && functionName == "-":
			return fmt.Sprintf(`help() %s`, msg.Msg)
		case functionName == "-":
			return fmt.Sprintf(`%s. %s`, msg.Object, msg.Msg)
		}
		return fmt.Sprintf(`%s.%s(%s) %s`, msg.Object, msg.Function, strings.Join(msg.Params, ", "), msg.Msg)
	}
	return ""
}

// AddExtensions takes an exisitng *otto.Otto (JavaScript VM) and adds os and http objects wrapping some Go native packages
func (js *JavaScriptVM) AddExtensions() *otto.Otto {
	errorObject := func(obj *otto.Object, msg string) otto.Value {
		if obj == nil {
			obj, _ = js.VM.Object(`({})`)
		}
		log.Println(msg)
		obj.Set("status", "error")
		obj.Set("error", msg)
		return obj.Value()
	}

	responseObject := func(data interface{}) otto.Value {
		src, _ := json.Marshal(data)
		obj, _ := js.VM.Object(fmt.Sprintf(`(%s)`, src))
		return obj.Value()
	}

	js.SetHelp("-", "-", []string{}, `Display usage information by object and function names. E.g. help(), help("os"), help("os","exit")`)
	js.VM.Set("help", func(call otto.FunctionCall) otto.Value {
		var (
			objectName   = "-"
			functionName = "-"
		)
		switch {
		case len(call.ArgumentList) == 1:
			objectName = call.Argument(0).String()
		default:
			objectName = call.Argument(0).String()
			functionName = call.Argument(1).String()
		}
		text := js.GetHelp(objectName, functionName)
		result, _ := js.VM.ToValue(text)
		return result
	})

	js.SetHelp("os", "-", []string{}, "Provides accession to operating systems function like reading/writing files")
	osObj, _ := js.VM.Object(`os = {}`)

	js.SetHelp("os", "args", []string{}, "Exposes any command line arguments left after flag.Parse() has run.")
	// os.args() returns an array of command line args after flag.Parse() has occurred.
	osObj.Set("args", func(call otto.FunctionCall) otto.Value {
		var args []string
		if flag.Parsed() == true {
			args = flag.Args()
		} else {
			args = os.Args
		}
		results, _ := js.VM.ToValue(args)
		return results
	})

	js.SetHelp("os", "exit", []string{"exitCode int"}, "Stops the program existing with the numeric value given(e.g. zero if everything is OK)")
	// os.exit()
	osObj.Set("exit", func(call otto.FunctionCall) otto.Value {
		exitCode := 0
		if len(call.ArgumentList) == 1 {
			s := call.Argument(0).String()
			exitCode, _ = strconv.Atoi(s)
		}
		os.Exit(exitCode)
		return responseObject(exitCode)
	})

	js.SetHelp("os", "getEnv", []string{"envvar string"}, `Gets the environment variable matching the structing. (e.g. os.getEnv(\"HOME\")`)
	// os.getEnv(env_varname) returns empty string or the value found as a string
	osObj.Set("getEnv", func(call otto.FunctionCall) otto.Value {
		envvar := call.Argument(0).String()
		result, err := js.VM.ToValue(os.Getenv(envvar))
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.getEnv(%q), %s", call.CallerLocation(), envvar, err))
		}
		return result
	})

	js.SetHelp("os", "readFile", []string{"filepath"}, "Reads the filename provided and returns the results as a JavaScript string")
	// os.readFile(filepath) returns the content of the filepath or empty string
	osObj.Set("readFile", func(call otto.FunctionCall) otto.Value {
		filename := call.Argument(0).String()
		buf, err := ioutil.ReadFile(filename)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.readFile(%q), %s", call.CallerLocation(), filename, err))
		}
		result, err := js.VM.ToValue(fmt.Sprintf("%s", buf))
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.readFile(%q), %s", call.CallerLocation(), filename, err))
		}
		return result
	})

	js.SetHelp("os", "writeFile", []string{"filepath string", "content string"}, "Writes a file, parameters are filepath and contents which are both strings")
	// os.writeFile(filepath, contents) returns true on sucess, false on failure
	osObj.Set("writeFile", func(call otto.FunctionCall) otto.Value {
		filename := call.Argument(0).String()
		buf := call.Argument(1).String()
		err := ioutil.WriteFile(filename, []byte(buf), 0660)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.writeFile(%q, %q), %s", call.CallerLocation(), filename, buf, err))
		}
		result, err := js.VM.ToValue(buf)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.writeFile(%q, %q), %s", call.CallerLocation(), filename, buf, err))
		}
		return result
	})

	js.SetHelp("os", "rename", []string{"oldpath string", "newpath string"}, "Renames oldpath to newpath")
	// os.rename(oldpath, newpath) renames a path returns an error object or true on success
	osObj.Set("rename", func(call otto.FunctionCall) otto.Value {
		oldpath := call.Argument(0).String()
		newpath := call.Argument(1).String()
		err := os.Rename(oldpath, newpath)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.rename(%q, %q), %s", call.CallerLocation(), oldpath, newpath, err))
		}
		result, _ := js.VM.ToValue(true)
		return result
	})

	js.SetHelp("os", "remove", []string{"filepath string"}, "Removes the file indicated by filepath")
	// os.remove(filepath) returns an error object or true if successful
	osObj.Set("remove", func(call otto.FunctionCall) otto.Value {
		pathname := call.Argument(0).String()
		fp, err := os.Open(pathname)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.remove(%q), %s", call.CallerLocation(), pathname, err))
		}
		defer fp.Close()
		stat, err := fp.Stat()
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.remove(%q), %s", call.CallerLocation(), pathname, err))
		}
		result, _ := js.VM.ToValue(false)
		if stat.IsDir() == false {
			err := os.Remove(pathname)
			if err != nil {
				return errorObject(nil, fmt.Sprintf("%s os.remove(%q), %s", call.CallerLocation(), pathname, err))
			}
			result, _ = js.VM.ToValue(true)
		}
		return result
	})

	js.SetHelp("os", "chmod", []string{"filepath string", "perms numeric"}, "Sets the permissions for a file (e.g. 0775, 0664)")
	// os.chmod(filepath, perms) returns an error object or true if successful
	osObj.Set("chmod", func(call otto.FunctionCall) otto.Value {
		filename := call.Argument(0).String()
		perms := call.Argument(1).String()

		fp, err := os.Open(filename)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.chmod(%q, %s), %s", call.CallerLocation(), filename, perms, err))
		}
		defer fp.Close()

		perm, err := strconv.ParseUint(perms, 10, 32)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.chmod(%q, %s), %s", call.CallerLocation(), filename, perms, err))
		}
		err = fp.Chmod(os.FileMode(perm))
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.chmod(%q, %s), %s", call.CallerLocation(), filename, perms, err))
		}
		result, _ := js.VM.ToValue(true)
		return result
	})

	js.SetHelp("os", "find", []string{"startpath string"}, "Looks for a files in startpath")
	// os.find(startpath) returns an array of path names
	osObj.Set("find", func(call otto.FunctionCall) otto.Value {
		var dirs []string
		startpath := call.Argument(0).String()
		err := filepath.Walk(startpath, func(p string, info os.FileInfo, err error) error {
			dirs = append(dirs, p)
			return err
		})
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.find(%q), %s", call.CallerLocation(), startpath, err))
		}
		result, err := js.VM.ToValue(dirs)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.find(%q), %s", call.CallerLocation(), startpath, err))
		}
		return result
	})

	js.SetHelp("os", "mkdir", []string{"pathname string", "perms numeric"}, "Makes a directory with the permissions (e.g. 0775)")
	// os.mkdir(pathname, perms) return an error object or true
	osObj.Set("mkdir", func(call otto.FunctionCall) otto.Value {
		newpath := call.Argument(0).String()
		perms := call.Argument(1).String()

		perm, err := strconv.ParseUint(perms, 10, 32)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.mkdir(%q, %s), %s", call.CallerLocation(), newpath, perms, err))
		}
		err = os.Mkdir(newpath, os.FileMode(perm))
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.mkdir(%q, %s), %s", call.CallerLocation(), newpath, perms, err))
		}

		result, _ := js.VM.ToValue(true)
		return result
	})

	js.SetHelp("os", "mkdirAll", []string{"pathname string", "perms numeric"}, "Makes a directory including missing ones in the path. E.g mkdir -p in Unix shell")
	// os.mkdir(pathname, perms) return an error object or true
	osObj.Set("mkdirAll", func(call otto.FunctionCall) otto.Value {
		newpath := call.Argument(0).String()
		perms := call.Argument(1).String()

		perm, err := strconv.ParseUint(perms, 10, 32)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.mkdir(%q, %s), %s", call.CallerLocation(), newpath, perms, err))
		}
		err = os.MkdirAll(newpath, os.FileMode(perm))
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.mkdir(%q, %s), %s", call.CallerLocation(), newpath, perms, err))
		}

		result, _ := js.VM.ToValue(true)
		return result
	})

	js.SetHelp("os", "rmdir", []string{"pathname string"}, "Removes the directory specified with pathname")
	// os.rmdir(pathname) returns an error object or true if successful
	osObj.Set("rmdir", func(call otto.FunctionCall) otto.Value {
		pathname := call.Argument(0).String()
		// NOTE: make sure this is a directory and not a file
		fp, err := os.Open(pathname)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.rmdir(%q), %s", call.CallerLocation(), pathname, err))
		}
		defer fp.Close()
		stat, err := fp.Stat()
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.rmdir(%q), %s", call.CallerLocation(), pathname, err))
		}
		result, _ := js.VM.ToValue(false)
		if stat.IsDir() == true {
			err := os.Remove(pathname)
			if err != nil {
				return errorObject(nil, fmt.Sprintf("%s os.rmdir(%q), %s", call.CallerLocation(), pathname, err))
			}
			result, _ = js.VM.ToValue(true)
		}
		return result
	})

	js.SetHelp("os", "rmdirAll", []string{"pathname string"}, "Removes a directory and any included in pathname")
	// os.rmdirAll(pathname) returns an error object or true if successful
	osObj.Set("rmdirAll", func(call otto.FunctionCall) otto.Value {
		pathname := call.Argument(0).String()
		// NOTE: make sure this is a directory and not a file
		fp, err := os.Open(pathname)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.rmdirAll(%q), %s", call.CallerLocation(), pathname, err))
		}
		defer fp.Close()
		stat, err := fp.Stat()
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.rmdirAll(%q), %s", call.CallerLocation(), pathname, err))
		}
		result, _ := js.VM.ToValue(false)
		if stat.IsDir() == true {
			err := os.RemoveAll(pathname)
			if err != nil {
				return errorObject(nil, fmt.Sprintf("%s os.rmdirAll(%q), %s", call.CallerLocation(), pathname, err))
			}
			result, _ = js.VM.ToValue(true)
		}
		return result
	})

	js.SetHelp("http", "-", []string{}, "The http object provides synchronous GET and POST methods")
	httpObj, _ := js.VM.Object(`http = {}`)

	js.SetHelp("http", "get", []string{"uri string", "headers []object"}, "performs a synchronous http GET operation")
	// http.Get(uri, headers) returns contents recieved (if any)
	httpObj.Set("get", func(call otto.FunctionCall) otto.Value {
		var headers []map[string]string

		uri := call.Argument(0).String()
		if len(call.ArgumentList) > 1 {
			rawObjs, err := call.Argument(1).Export()
			if err != nil {
				return errorObject(nil, fmt.Sprintf("Failed to process headers, %s, %s, %s", call.CallerLocation(), uri, err))
			}
			src, _ := json.Marshal(rawObjs)
			err = json.Unmarshal(src, &headers)
			if err != nil {
				return errorObject(nil, fmt.Sprintf("Failed to translate headers, %s, %s, %s", call.CallerLocation(), uri, err))
			}
		}

		client := &http.Client{}
		req, err := http.NewRequest("GET", uri, nil)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("Can't create a GET request for %s, %s, %s", uri, call.CallerLocation(), err))
		}
		for _, header := range headers {
			for k, v := range header {
				req.Header.Set(k, v)
			}
		}
		resp, err := client.Do(req)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("Can't connect to %s, %s, %s", uri, call.CallerLocation(), err))
		}
		defer resp.Body.Close()
		content, err := ioutil.ReadAll(resp.Body)

		result, err := js.VM.ToValue(fmt.Sprintf("%s", content))
		if err != nil {
			return errorObject(nil, fmt.Sprintf("http.get(%q, headers) error, %s, %s", uri, call.CallerLocation(), err))
		}
		return result
	})

	js.SetHelp("http", "post", []string{"uri string", "headers []object", "payload string"}, "Performs a synchronous http POST operation")
	// HttpPost(uri, headers, payload) returns contents recieved (if any)
	httpObj.Set("post", func(call otto.FunctionCall) otto.Value {
		var headers []map[string]string

		uri := call.Argument(0).String()
		mimeType := call.Argument(1).String()
		payload := call.Argument(2).String()
		buf := strings.NewReader(payload)
		// Process any additional headers past to http.Post()
		if len(call.ArgumentList) > 2 {
			rawObjs, err := call.Argument(3).Export()
			if err != nil {
				return errorObject(nil, fmt.Sprintf("Failed to process headers for %s, %s, %s", uri, call.CallerLocation(), err))
			}
			src, _ := json.Marshal(rawObjs)
			err = json.Unmarshal(src, &headers)
			if err != nil {
				return errorObject(nil, fmt.Sprintf("Failed to translate header for %s, %s, %s", uri, call.CallerLocation(), err))
			}
		}

		client := &http.Client{}
		req, err := http.NewRequest("POST", uri, buf)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("Can't create a POST request for %s, %s, %s", uri, call.CallerLocation(), err))
		}
		req.Header.Set("Content-Type", mimeType)
		for _, header := range headers {
			for k, v := range header {
				req.Header.Set(k, v)
			}
		}
		resp, err := client.Do(req)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("Can't connect to %s, %s, %s", uri, call.CallerLocation(), err))
		}
		defer resp.Body.Close()
		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("Can't read response %s, %s, %s", uri, call.CallerLocation(), err))
		}
		result, err := js.VM.ToValue(fmt.Sprintf("%s", content))
		if err != nil {
			return errorObject(nil, fmt.Sprintf("http.post(%q, headers, payload) error, %s, %s", uri, call.CallerLocation(), err))
		}
		return result
	})

	script, err := js.VM.Compile("polyfill", Polyfill)
	if err != nil {
		log.Fatalf("polyfill compile error: %s\n\n%s\n", err, Polyfill)
	}
	js.VM.Eval(script)
	return js.VM
}

// Provide a Repl for working with IxIF content via JavaScript
// args holds the command line parameters passed to the repl for processing by a script in interactively in the repl.
// Returns an integer value suitable to pass to os.Exit().
func (js *JavaScriptVM) Repl() {

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir, _ = filepath.Abs(".")
	}
	rl, err := readline.NewEx(&readline.Config{
		Prompt:       "> ",
		HistoryFile:  path.Join(homeDir, ".ottomatic_history"),
		AutoComplete: js.AutoCompleter,
		// for multi-line support see https://github.com/chzyer/readline/blob/master/example/readline-multiline/readline-multiline.go
		//DisableAutoSaveHistory: true,
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	for i := 1; true; i++ {
		line, err := rl.Readline()
		if err != nil { // io.EOF, readline.ErrInterrupt
			break
		}
		script, _ := js.VM.Compile(fmt.Sprintf("command %d", i), line)
		js.VM.Eval(script)
	}
}

//
// This is extenion to the original otto
//

// ToStruct will attempt populate a struct passed in as a parameter.
//
// ToStruct returns an error if it runs into a problem.
//
// Example:
// a := struct{One int, Two string}{}
// val, _ := vm.Run(`(function (){ return {One: 1, Two: "two"}}())`)
// _ := val.ToSruct(&a)
// fmt.Printf("One: %d, Two: %s\n", a.One, a.Two)
//
func ToStruct(value otto.Value, aStruct interface{}) error {
	raw, err := value.Export()
	if err != nil {
		return fmt.Errorf("failed to export value, %s", err)
	}
	src, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("failed to marshal value, %s", err)
	}
	err = json.Unmarshal(src, &aStruct)
	if err != nil {
		return fmt.Errorf("failed to unmarshal value, %s", err)
	}
	return nil
}
