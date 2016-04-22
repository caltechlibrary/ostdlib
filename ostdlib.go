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
	"bytes"
	"encoding/json"
	"encoding/xml"
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
	"github.com/fatih/color"
	"github.com/robertkrimen/otto"
	"github.com/tealeg/xlsx"
)

// Version of the Otto Standard Library
const Version = "0.0.7"

var (
	// Workbookfill wraps the xlsx object to provide a more wholistic worbook experience
	Workbookfill = `
	xlsx.New = function (val) {
    	if (val === undefined) {
			val = {};
		}
		if (val.valueOf !== undefined) {
			val = val.valueOf();
		}
		return {
			__data: val,
			read: function (name) {
				var data = xlsx.read(name);
			  	if (data) {
					 this.__data = data;
					 return true;
				}
				return false;
  			},
			write: function (name) {
				return xlsx.write(name, this.__data)
			},
			getSheetNames: function () {
				return Object.keys(this.__data);
			},
			getSheet: function(name) {
				if (this.__data[name] === undefined) {
					return null;
				}
				return this.__data[name];
			},
			setSheet: function(name, sheet) {
				return (this.__data[name] = sheet);
			},
			getSheetNo: function (sheetNo) {
				var names = Object.keys(this.__data);
				if (sheetNo >= 0 && sheetNo < names.length) {
					return this.getSheet(names[sheetNo]);
				}
				return null;
			},
			setSheetNo: function (sheetNo, sheet) {
				var names = Object.keys(this.__data);
				if (sheetNo >= 0 && sheetNo < names.length) {
					return this.setSheet(names[sheetNo], sheet);
				}
				return this.setSheet('Untitled Sheet '+sheetNo, sheet);
			},
			valueOf: function () {
				return this.__data;
			},
			toString: function() {
				return JSON.stringify(this.__data);
			}
		};
	};
	var Workbook = xlsx.New();
`
	// Polyfill addes missing functionality implemented in JavaScript rather than Go
	Polyfill = `
	if (!Array.prototype.copyWithin) {
		Array.prototype.copyWithin = function(target, start/*, end*/) {
			// Steps 1-2.
			if (this == null) {
				throw new TypeError('this is null or not defined');
			}

			var O = Object(this);

			// Steps 3-5.
			var len = O.length >>> 0;

			// Steps 6-8.
			var relativeTarget = target >> 0;

			var to = relativeTarget < 0 ?
			Math.max(len + relativeTarget, 0) :
			Math.min(relativeTarget, len);

			// Steps 9-11.
			var relativeStart = start >> 0;

			var from = relativeStart < 0 ?
			Math.max(len + relativeStart, 0) :
			Math.min(relativeStart, len);

			// Steps 12-14.
			var end = arguments[2];
			var relativeEnd = end === undefined ? len : end >> 0;

			var final = relativeEnd < 0 ?
			Math.max(len + relativeEnd, 0) :
			Math.min(relativeEnd, len);

			// Step 15.
			var count = Math.min(final - from, len - to);

			// Steps 16-17.
			var direction = 1;

			if (from < to && to < (from + count)) {
				direction = -1;
				from += count - 1;
				to += count - 1;
			}

			// Step 18.
			while (count > 0) {
				if (from in O) {
					O[to] = O[from];
				} else {
					delete O[to];
				}

				from += direction;
				to += direction;
				count--;
			}

			// Step 19.
			return O;
		};
	}
	if (typeof Object.create !== 'function') {
	  Object.create = (function() {
	    var Temp = function() {};
	    return function (prototype) {
	      if (arguments.length > 1) {
	        throw Error('Second argument not supported');
	      }
	      if(prototype !== Object(prototype) && prototype !== null) {
	        throw TypeError('Argument must be an object or null');
	     }
	     if (prototype === null) {
	        throw Error('null [[Prototype]] not supported');
	      }
	      Temp.prototype = prototype;
	      var result = new Temp();
	      Temp.prototype = null;
	      return result;
	    };
	  })();
	}
	if (typeof Object.defineProperties !== 'function') {
		Object.defineProperties = function (obj, properties) {
		  function convertToDescriptor(desc) {
		    function hasProperty(obj, prop) {
		      return Object.prototype.hasOwnProperty.call(obj, prop);
		    }

		    function isCallable(v) {
		      // NB: modify as necessary if other values than functions are callable.
		      return typeof v === "function";
		    }

		    if (typeof desc !== "object" || desc === null)
		      throw new TypeError("bad desc");

		    var d = {};

		    if (hasProperty(desc, "enumerable"))
		      d.enumerable = !!desc.enumerable;
		    if (hasProperty(desc, "configurable"))
		      d.configurable = !!desc.configurable;
		    if (hasProperty(desc, "value"))
		      d.value = desc.value;
		    if (hasProperty(desc, "writable"))
		      d.writable = !!desc.writable;
		    if (hasProperty(desc, "get")) {
		      var g = desc.get;

		      if (!isCallable(g) && typeof g !== "undefined")
		        throw new TypeError("bad get");
		      d.get = g;
		    }
		    if (hasProperty(desc, "set")) {
		      var s = desc.set;
		      if (!isCallable(s) && typeof s !== "undefined")
		        throw new TypeError("bad set");
		      d.set = s;
		    }

		    if (("get" in d || "set" in d) && ("value" in d || "writable" in d))
		      throw new TypeError("identity-confused descriptor");

		    return d;
		  }

		  if (typeof obj !== "object" || obj === null)
		    throw new TypeError("bad obj");

		  properties = Object(properties);

		  var keys = Object.keys(properties);
		  var descs = [];

		  for (var i = 0; i < keys.length; i++)
		    descs.push([keys[i], convertToDescriptor(properties[keys[i]])]);

		  for (var i = 0; i < descs.length; i++)
		    Object.defineProperty(obj, descs[i][0], descs[i][1]);

		  return obj;
		};
	}
	if (typeof Object.assign !== 'function') {
	    Object.assign = function (target) {
	      'use strict';
	      if (target === undefined || target === null) {
	        throw new TypeError('Cannot convert undefined or null to object');
	      }

	      var output = Object(target);
	      for (var index = 1; index < arguments.length; index++) {
	        var source = arguments[index];
	        if (source !== undefined && source !== null) {
	          for (var nextKey in source) {
	            if (source.hasOwnProperty(nextKey)) {
	              output[nextKey] = source[nextKey];
	            }
	          }
	        }
	      }
	      return output;
	    };
	}
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
	if (!Number.prototype.parseInt) {
		Number.prototype.parseInt = parseInt;
	}
	if (!Number.prototype.parseFloat) {
		Number.prototype.parseFloat = parseFloat;
	}
`
)

// HelpMsg supports storing interactive help content
type HelpMsg struct {
	XMLName  xml.Name `xml:"HelpMsg" json:"-"`
	Object   string   `xml:"object" json:"object"`
	Function string   `xml:"function" json:"function"`
	Params   []string `xml:"parameters" json:"parameters"`
	Msg      string   `xml:"docstring" json:"docstring"`
}

// JavaScriptVM is a wrapper for *otto.Otto to make it easy to add features without forking Otto.
type JavaScriptVM struct {
	VM                *otto.Otto
	AutoCompleter     *readline.PrefixCompleter
	AutoCompleteTerms []string              `xml:"autocomplete_terms" json:"autocomplete_terms"`
	Help              map[string][]*HelpMsg `xml:"help" json:"help"`
}

// PrintDefaultWelcome display default weclome message based on
// JavaScriptVM.HelpMsg
func (js *JavaScriptVM) PrintDefaultWelcome() {
	bold := color.New(color.Bold).SprintFunc()
	appName := path.Base(os.Args[0])
	fmt.Printf(" Welcome to %s\n\n", bold(appName))
	fmt.Printf(" Type %s to exit or %s for help information\n (e.g. %s or %s)\n\n", bold(".exit"), bold(".help"), bold(".help os"), bold(".help os.exit"))
	fmt.Println(" Help is available for the following objects.")
	for k := range js.Help {
		fmt.Printf("\t%s", bold(k))
	}
	fmt.Println("")
	if js.AutoCompleter != nil {
		fmt.Println(" Press tab for auto completion")
	}
	fmt.Printf(" repl version %s\n\n", Version)
}

// New create a new JavaScriptVM structure extending the functionality of *otto.Otto
func New(vm *otto.Otto) *JavaScriptVM {
	js := new(JavaScriptVM)
	js.VM = vm
	js.Help = make(map[string][]*HelpMsg)

	js.AutoCompleter = readline.NewPrefixCompleter()
	return js
}

// SetHelp adds help documentation by object and function
func (js *JavaScriptVM) SetHelp(objectName string, functionName string, params []string, text string) {
	if objectName == "" {
		return
	}
	msg := new(HelpMsg)
	msg.Object = objectName
	msg.Function = functionName
	msg.Params = params
	msg.Msg = text

	var name string
	if len(msg.Params) == 0 {
		name = fmt.Sprintf(`%s.%s()`, msg.Object, msg.Function)
	} else {
		name = fmt.Sprintf(`%s.%s(%s)`, msg.Object, msg.Function, strings.Join(msg.Params, ", "))
	}
	js.AutoCompleteTerms = append(js.AutoCompleteTerms, name)

	if data, ok := js.Help[objectName]; ok == true {
		data = append(data, msg)
		js.Help[objectName] = data
		return
	}
	var data []*HelpMsg
	data = append(data, msg)
	js.Help[objectName] = data
}

// GetHelp retrieves help text by object and function names
func (js *JavaScriptVM) GetHelp(objectName, functionName string) {
	bold := color.New(color.Bold).SprintFunc()
	if objectName == "" {
		s := []string{"help provides information about objects and functions"}
		for ky := range js.Help {
			s = append(s, ky)
		}
		fmt.Printf("%s\n", strings.Join(s, "\n   "))
		fmt.Println("Additionally the repl provide the following dot commands")
		fmt.Printf(" %s\tshow help\n", bold(".help"))
		fmt.Printf(" %s\tbreak out multi-line entry without saving command\n", bold(".break"))
		fmt.Printf(" %s\texit repl\n", bold(".exit"))
		fmt.Printf(" %s\tlist history\n", bold(".list"))
		fmt.Printf(" %s FILENAME\tload history from FILENAME\n", bold(".load"))
		fmt.Printf(" %s\ttrunctate history\n", bold(".reset"))
		fmt.Printf(" %s FILENAME\tsave history to FILENAME\n", bold(".save"))
		return
	}
	s := []string{fmt.Sprintf("%s", objectName)}
	if topics, ok := js.Help[objectName]; ok == true {
		for _, msg := range topics {
			if functionName == "" {
				t := fmt.Sprintf(`%s.%s(%s)`, msg.Object, msg.Function, strings.Join(msg.Params, ", "))
				s = append(s, t)
			} else if functionName == msg.Function {
				t := fmt.Sprintf("%s.%s(%s)\n    %s", msg.Object, msg.Function, strings.Join(msg.Params, ", "), msg.Msg)
				s = append(s, t)
			}
		}
	}
	fmt.Printf("%s\n", strings.Join(s, "\n  "))
	return
}

// AddAutoComplete populates the auto completion based on the help data structure
func (js *JavaScriptVM) AddAutoComplete() {
	completer := readline.NewPrefixCompleter()
	children := completer.GetChildren()
	children = append(children, readline.PcItem(".help"))
	children = append(children, readline.PcItem(".break"))
	children = append(children, readline.PcItem(".exit"))
	children = append(children, readline.PcItem(".list"))
	children = append(children, readline.PcItem(".load"))
	children = append(children, readline.PcItem(".reset"))
	children = append(children, readline.PcItem(".save"))
	for _, text := range js.AutoCompleteTerms {
		children = append(children, readline.PcItem(text))
	}
	completer.SetChildren(children)
	js.AutoCompleter = completer
}

// AddHelp adds the interactive help based on the extensions defined in ostdlib
func (js *JavaScriptVM) AddHelp() {
	js.SetHelp("os", "args", []string{}, "Exposes any command line arguments left after flag.Parse() has run.")
	js.SetHelp("os", "exit", []string{"exitCode int, log_msg string"}, "Stops the program existing with the numeric value given(e.g. zero if everything is OK), an optional log message can be included.")
	js.SetHelp("os", "getEnv", []string{"envvar string"}, `Gets the environment variable matching the structing. (e.g. os.getEnv(\"HOME\")`)
	js.SetHelp("os", "setEnv", []string{"envvar string"}, `Sets the environment variable. (e.g. os.setEnv(\"Welcome\", \"Hi there\")`)
	js.SetHelp("os", "readFile", []string{"filepath"}, "Reads the filename provided and returns the results as a JavaScript string")
	js.SetHelp("os", "writeFile", []string{"filepath string", "content string"}, "Writes a file, parameters are filepath and contents which are both strings")
	js.SetHelp("os", "rename", []string{"oldpath string", "newpath string"}, "Renames oldpath to newpath")
	js.SetHelp("os", "remove", []string{"filepath string"}, "Removes the file indicated by filepath")
	js.SetHelp("os", "chmod", []string{"filepath string", "perms numeric"}, "Sets the permissions for a file (e.g. 0775, 0664)")
	js.SetHelp("os", "find", []string{"startpath string"}, "Looks for a files in startpath")
	js.SetHelp("os", "mkdir", []string{"pathname string", "perms numeric"}, "Makes a directory with the permissions (e.g. 0775)")
	js.SetHelp("os", "mkdirAll", []string{"pathname string", "perms numeric"}, "Makes a directory including missing ones in the path. E.g mkdir -p in Unix shell")
	js.SetHelp("os", "rmdir", []string{"pathname string"}, "Removes the directory specified with pathname")
	js.SetHelp("os", "rmdirAll", []string{"pathname string"}, "Removes a directory and any included in pathname")
	js.SetHelp("http", "get", []string{"uri string", "headers []object"}, "performs a synchronous http GET operation")
	js.SetHelp("http", "post", []string{"uri string", "headers []object", "payload string"}, "Performs a synchronous http POST operation")
	js.SetHelp("xlsx", "read", []string{"filename string"}, "Reads in an Excel xlsx workbook file and returns an object contains the sheets found or error object")
	js.SetHelp("xlsx", "write", []string{"filename string, sheetObject object"}, "Write an Excel xlsx workbook file and returns true on success or error object")
	js.SetHelp("xlsx", "New", []string{}, "Constructor for Workbook object")
	// Help for JavaScript native Workbook object that wraps xlsx
	js.SetHelp("Workbook", "read", []string{"filename string"}, "reads an xlsx file into the workbook")
	js.SetHelp("Workbook", "write", []string{"filename string"}, "write an xlsx file from the workbook")
	js.SetHelp("Workbook", "getSheetNames", []string{}, "returns an array of names of the spreadsheets in a workbook")
	js.SetHelp("Workbook", "getSheet", []string{"name string"}, "get the individual spreadsheet by name from the workbook")
	js.SetHelp("Workbook", "setSheet", []string{"name string", "sheet is a 2D array of rows and cells"}, "set a spreadsheet by name to the rows and cell defined by sheet")
	js.SetHelp("Workbook", "getSheetNo", []string{"sheetNo int"}, "get the individual spreadsheet by sheet no. from the workbook")
	js.SetHelp("Workbook", "setSheetNo", []string{"sheetNo", "sheet is a 2D array of rows and cells"}, "set a spreadsheet by sheet no. to the rows and cell defined by sheet")
	js.SetHelp("Workbook", "valueOf", []string{}, "returns the __data attribute of the workbook")
	js.SetHelp("Workbook", "toString", []string{}, "returns a JSON view of __data attribute of the workbook")
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

	osObj, _ := js.VM.Object(`os = {}`)

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

	// os.exit()
	osObj.Set("exit", func(call otto.FunctionCall) otto.Value {
		exitCode := 0
		if len(call.ArgumentList) >= 1 {
			s := call.Argument(0).String()
			exitCode, _ = strconv.Atoi(s)
		}
		if len(call.ArgumentList) == 2 {
			log.Println(call.Argument(1).String())
		}
		os.Exit(exitCode)
		return responseObject(exitCode)
	})

	// os.getEnv(env_varname) returns empty string or the value found as a string
	osObj.Set("getEnv", func(call otto.FunctionCall) otto.Value {
		envvar := call.Argument(0).String()
		result, err := js.VM.ToValue(os.Getenv(envvar))
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.getEnv(%q), %s", call.CallerLocation(), envvar, err))
		}
		return result
	})

	// os.setEnv(env_varname, value) sets the environment variable for the session, returns the value set.
	osObj.Set("setEnv", func(call otto.FunctionCall) otto.Value {
		envvar := call.Argument(0).String()
		val := call.Argument(1).String()
		err := os.Setenv(envvar, val)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.setEnv(%q, %q), %s", call.CallerLocation(), envvar, val, err))
		}
		result, err := js.VM.ToValue(os.Getenv(envvar))
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.setEnv(%q, %q), %s", call.CallerLocation(), envvar, val, err))
		}
		return result
	})

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

	httpObj, _ := js.VM.Object(`http = {}`)

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

	// workbook wraps github.com/tealeg/xlsx library making it easy to read/write Excel xlsx files from Otto
	workbook, _ := js.VM.Object(`xlsx = {}`)
	// Workbook.read(filename) returns an object with properties of sheet names pointing at 2d-arrays of strings or error object
	workbook.Set("read", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) != 1 {
			return errorObject(nil, fmt.Sprintf("xlxs.read(filename), error missing filename, %s", call.CallerLocation()))
		}
		fname := call.Argument(0).String()
		xlWorkbook, err := xlsx.OpenFile(fname)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("xlsx.read(%q), error %s, %s", fname, call.CallerLocation(), err))
		}
		var markup []string

		// Start Workbook object markup
		markup = append(markup, fmt.Sprintf("{"))
		for i, sheet := range xlWorkbook.Sheets {
			if i > 0 {
				markup = append(markup, fmt.Sprintf(","))
			}
			// Start a sheet with sheetNameString
			markup = append(markup, fmt.Sprintf("%q:[", sheet.Name))
			for j, row := range sheet.Rows {
				if j > 0 {
					markup = append(markup, fmt.Sprintf(","))
				}
				// Start Row of cells
				markup = append(markup, fmt.Sprintf("["))
				for k, cell := range row.Cells {
					if k > 0 {
						markup = append(markup, fmt.Sprintf(","))
					}
					//NOTE: could use cell.Type() to convert to JS formatted values instead of forcing to a string
					s, _ := cell.String()
					markup = append(markup, fmt.Sprintf("%q", s))
				}
				// Close Row of cells
				markup = append(markup, fmt.Sprintf("]"))
			}
			// Close a sheet
			markup = append(markup, fmt.Sprintf("]"))
		}
		// End Workbook object markup
		markup = append(markup, fmt.Sprintf("}"))
		result, err := js.VM.Eval(fmt.Sprintf("(function (){ return %s;}());", strings.Join(markup, "")))
		if err != nil {
			return errorObject(nil, fmt.Sprintf("xlsx.read(%q) error, %s, %s", fname, call.CallerLocation(), err))
		}
		return result
	})

	// Workbook.write(filename, sheetObject) returns true on success, false otherwise. sheetObject should have properties of sheet names pointing at a 2d array of strings
	workbook.Set("write", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) != 2 {
			return errorObject(nil, fmt.Sprintf("xlsx.write(filename, sheetsObject), missing parameters, %s", call.CallerLocation()))
		}
		fname := call.Argument(0).String()
		data, err := call.Argument(1).Export()
		if err != nil {
			return errorObject(nil, fmt.Sprintf("xlsx.write(%q, sheetsObject), error %s, %s", fname, call.CallerLocation(), err))
		}
		var file *xlsx.File

		file = xlsx.NewFile()
		for sheetName, table := range data.(map[string]interface{}) {
			sheet, err := file.AddSheet(sheetName)
			if err != nil {
				log.Printf("%s, can't add sheet %s, %s", fname, sheetName, err)
			} else {
				for _, tr := range table.([][]string) {
					row := sheet.AddRow()
					for _, td := range tr {
						cell := row.AddCell()
						cell.Value = td
					}
				}
			}
		}
		err = file.Save(fname)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("xlsx.write(%q, sheetsObject), error %s, %s", fname, call.CallerLocation(), err))
		}
		result, err := js.VM.ToValue(true)
		if err != nil {
			return errorObject(nil, fmt.Sprintf("xlsx.write(%q, sheetsObject) error, %s, %s", fname, call.CallerLocation(), err))
		}
		return result
	})
	script, err := js.VM.Compile("workbookfill", Workbookfill)
	if err != nil {
		log.Fatalf("Workbookfill compile error: %s\n\n%s\n", err, Workbookfill)
	}
	js.VM.Eval(script)

	script, err = js.VM.Compile("polyfill", Polyfill)
	if err != nil {
		log.Fatalf("polyfill compile error: %s\n\n%s\n", err, Polyfill)
	}
	js.VM.Eval(script)
	return js.VM
}

// Runner given a list of JavaScript filenames run the files
func (js *JavaScriptVM) Runner(filenames []string) {
	for _, fname := range filenames {
		src, err := ioutil.ReadFile(fname)
		if err != nil {
			log.Fatalf("Can't read file %s, %s", fname, err)
		}
		script, err := js.VM.Compile(fname, src)
		if err != nil {
			log.Fatalf("%s", err)
		}
		_, err = js.VM.Eval(script)
		if err != nil {
			log.Fatalf("%s", err)
		}
	}
}

// Repl provides interactive JavaScript shell supporting autocomplete and command history
func (js *JavaScriptVM) Repl() {
	bold := color.New(color.Bold).SprintFunc()

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir, _ = filepath.Abs(".")
	}
	historyFileName := fmt.Sprintf(".%s_history", path.Base(os.Args[0]))
	rl, err := readline.NewEx(&readline.Config{
		Prompt:       "> ",
		HistoryFile:  path.Join(homeDir, historyFileName),
		AutoComplete: js.AutoCompleter,
		// for multi-line support see https://github.com/chzyer/readline/blob/master/example/readline-multiline/readline-multiline.go
		DisableAutoSaveHistory: true,
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	var cmds []string
	for i := 1; true; i++ {
		line, err := rl.Readline()
		if err != nil { // io.EOF, readline.ErrInterrupt
			break
		}
		switch {
		case strings.HasPrefix(line, ".help"):
			topic := strings.TrimPrefix(line, ".help ")
			if topic == "" {
				js.GetHelp("", "")
			} else {
				dotPosition := strings.Index(topic, ".")
				if dotPosition > -1 && dotPosition <= len(topic) {
					o := topic[0:dotPosition]
					f := topic[dotPosition+1:]
					js.GetHelp(o, f)
				} else {
					js.GetHelp(topic, "")
				}
			}
		case strings.HasPrefix(line, ".list"):
			buf, err := ioutil.ReadFile(rl.Config.HistoryFile)
			if err != nil {
				fmt.Printf("History is readable, %s\n", err)
				break
			}
			fmt.Printf("%s", buf)
		case strings.HasPrefix(line, ".load"):
			s := strings.SplitN(line, " ", 2)
			if len(s) < 2 || s[1] == "" {
				js.GetHelp("", "")
				break
			}
			buf, err := ioutil.ReadFile(s[1])
			if err != nil {
				fmt.Printf("History is readable, %s\n", err)
				break
			}
			for _, b := range bytes.Split(buf, []byte("\n")) {
				rl.SaveHistory(fmt.Sprintf("%s", b))
			}
			fmt.Printf("%s loaded\n", s[1])
		case strings.HasPrefix(line, ".reset"):
			err := os.Truncate(rl.Config.HistoryFile, 0)
			if err != nil {
				fmt.Printf("Could not truncate history, %s\n", err)
				break
			}
			fmt.Println("history truncated")
		case strings.HasPrefix(line, ".save"):
			buf, err := ioutil.ReadFile(rl.Config.HistoryFile)
			if err != nil {
				fmt.Printf("History is readable, %s\n", err)
				break
			}
			s := strings.SplitN(line, " ", 2)
			if len(s) != 2 || s[1] == "" {
				js.GetHelp("", "")
				break
			}
			if err := ioutil.WriteFile(s[1], buf, 0600); err != nil {
				fmt.Printf("Can't write %s, %s", s[1], err)
				break
			}
			fmt.Printf(".save %s completed\n", s[1])
		case strings.HasPrefix(line, ".exit"):
			os.Exit(0)
		case line == ".break":
			fmt.Printf("Clearing input %q\n", strings.Join(cmds, " "))
			cmds = []string{}
			rl.SetPrompt("> ")
		default:
			cmds = append(cmds, line)
			script, err := js.VM.Compile(fmt.Sprintf("command %d", i), strings.Join(cmds, " "))
			if err != nil {
				fmt.Printf("%s\n", err)
				rl.SetPrompt(fmt.Sprintf("%0.2d: ", len(cmds)))
			} else {
				rl.SetPrompt("> ")
				rl.SaveHistory(strings.Join(cmds, " "))
				cmds = []string{}
				val, err := js.VM.Eval(script)
				if err != nil {
					fmt.Printf("js error: %s\n", err)
				}
				fmt.Printf("    %s\n", bold(val.String()))
			}
		}
	}
}

//
// This is an extenion to the original otto value methods
//

// ToStruct will attempt populate a struct passed in as a parameter.
//
// ToStruct returns an error if it runs into a problem.
//
// Example:
// a := struct{One int, Two string}{}
// val, _ := vm.Run(`(function (){ return {One: 1, Two: "two"}}())`)
// _ := ToSruct(val, &a)
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
