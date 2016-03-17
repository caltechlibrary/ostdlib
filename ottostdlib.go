//
// Package ottostdlib is a collection of JavaScript objects and functions for standardizing
// embedding the Otto JavaScript interpreter in Caltech Library Projects.
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
package ottostdlib

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	// 3rd Party Packages
	"github.com/caltechlibrary/otto"
)

// AddStdlib attaches various objects to an Otto JS VM
func AddStdLib(vm *otto.Otto) *otto.Otto {
	errorObject := func(obj *otto.Object, msg string) otto.Value {
		if obj == nil {
			obj, _ = vm.Object(`({})`)
		}
		log.Println(msg)
		obj.Set("status", "error")
		obj.Set("error", msg)
		return obj.Value()
	}

	responseObject := func(data interface{}) otto.Value {
		src, _ := json.Marshal(data)
		obj, _ := vm.Object(fmt.Sprintf(`(%s)`, src))
		return obj.Value()
	}

	osObj, _ := vm.Object(`os = {}`)

	// os.args() returns an array of command line args
	osObj.Set("args", func(call otto.FunctionCall) otto.Value {
		var args []string
		if flag.Parsed() == true {
			args = flag.Args()
		} else {
			args = os.Args
		}
		results, _ := vm.ToValue(args)
		return results
	})

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

	// os.getEnv(env_varname) returns empty string or the value found as a string
	osObj.Set("getEnv", func(call otto.FunctionCall) otto.Value {
		envvar := call.Argument(0).String()
		result, err := vm.ToValue(os.Getenv(envvar))
		if err != nil {
			return errorObject(nil, fmt.Sprintf("%s os.getEnv(%q), %s", call.CallerLocation(), envvar, err))
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
		result, err := vm.ToValue(fmt.Sprintf("%s", buf))
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
		result, err := vm.ToValue(buf)
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
		result, _ := vm.ToValue(true)
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
		result, _ := vm.ToValue(false)
		if stat.IsDir() == false {
			err := os.Remove(pathname)
			if err != nil {
				return errorObject(nil, fmt.Sprintf("%s os.remove(%q), %s", call.CallerLocation(), pathname, err))
			}
			result, _ = vm.ToValue(true)
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
		result, _ := vm.ToValue(true)
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
		result, err := vm.ToValue(dirs)
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

		result, _ := vm.ToValue(true)
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

		result, _ := vm.ToValue(true)
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
		result, _ := vm.ToValue(false)
		if stat.IsDir() == true {
			err := os.Remove(pathname)
			if err != nil {
				return errorObject(nil, fmt.Sprintf("%s os.rmdir(%q), %s", call.CallerLocation(), pathname, err))
			}
			result, _ = vm.ToValue(true)
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
		result, _ := vm.ToValue(false)
		if stat.IsDir() == true {
			err := os.RemoveAll(pathname)
			if err != nil {
				return errorObject(nil, fmt.Sprintf("%s os.rmdirAll(%q), %s", call.CallerLocation(), pathname, err))
			}
			result, _ = vm.ToValue(true)
		}
		return result
	})

	httpObj, _ := vm.Object(`http = {}`)

	//HttpGet(uri, headers) returns contents recieved (if any)
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
		if err != nil {
			return errorObject(nil, fmt.Sprintf("Can't read response %s, %s, %s", uri, call.CallerLocation(), err))
		}
		return responseObject(content)
	})

	// HttpPost(uri, headers, payload) returns contents recieved (if any)
	httpObj.Set("post", func(call otto.FunctionCall) otto.Value {
		var headers []map[string]string

		uri := call.Argument(0).String()
		mimeType := call.Argument(1).String()
		payload := call.Argument(2).String()
		buf := strings.NewReader(payload)
		// Process any additional headers past to HttpPost()
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
		result, err := vm.ToValue(fmt.Sprintf("%s", content))
		if err != nil {
			return errorObject(nil, fmt.Sprintf("HttpGet(%q) error, %s, %s", uri, call.CallerLocation(), err))
		}
		return result
	})

	//
	// Add JS Polyfills as needed
	//
	polyfil := `
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
	script, err := vm.Compile("polyfil", polyfil)
	if err != nil {
		log.Fatalf("polyfil compile error: %s\n\n%s\n", err, polyfil)
	}
	vm.Eval(script)
	return vm
}
