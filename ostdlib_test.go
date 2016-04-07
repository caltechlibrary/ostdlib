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

//
// This is extenion to the original otto
//
import (
	"fmt"
	"strings"
	"testing"

	// 3rd Party packages
	"github.com/robertkrimen/otto"
)

func isOK(t *testing.T, o1 interface{}, o2 interface{}) {
	switch o1.(type) {
	case string:
		s1 := fmt.Sprintf("%s", o1)
		s2 := fmt.Sprintf("%s", o2)
		if strings.Compare(s1, s2) != 0 {
			t.Errorf("strings %q != %q", o1, o2)
		}
	case int, int32, int64:
		if o1 != o2 {
			t.Errorf("int %d != %d", o1, o2)
		}
	case float32, float64:
		s1 := fmt.Sprintf("%f", o1)
		s2 := fmt.Sprintf("%f", o2)
		if strings.Compare(s1, s2) != 0 {
			t.Errorf("float %f != %f", o1, o2)
		}
	case bool:
		if o1 != o2 {
			t.Errorf("bool %T != %T", o1, o2)
		}
	default:
		if o1 != o2 {
			t.Errorf("unknown type %+v != %+v", o1, o2)
		}
	}
}

func TestToStructValue(t *testing.T) {
	vm := otto.New()
	jsSrc := `(function () {return {one: 1, two: "Two", three: 3.0, four: [1,2,3,4], five: true};}())`
	aStruct := struct {
		One   int     `json:"one"`
		Two   string  `json:"two"`
		Three float32 `json:"three"`
		Four  []int   `json:"four"`
		Five  bool    `json:"five"`
	}{}

	val, err := vm.Run(jsSrc)
	isOK(t, err, nil)
	err = ToStruct(val, &aStruct)
	isOK(t, err, nil)
	isOK(t, aStruct.One, 1)
	isOK(t, aStruct.Two, "Two")
	isOK(t, aStruct.Three, 3.0)
	isOK(t, len(aStruct.Four), 4)
	isOK(t, aStruct.Four[0], 1)
	isOK(t, aStruct.Four[1], 2)
	isOK(t, aStruct.Four[2], 3)
	isOK(t, aStruct.Four[3], 4)
	isOK(t, aStruct.Five, true)
}

func TestHelpSystem(t *testing.T) {
	vm := otto.New()
	js := New(vm)

	js.SetHelp("test", "help", []string{"one int", "two string"}, "test.help() example")
	js.AddHelp()
	js.AddAutoComplete()
	isOK(t, len(js.AutoCompleter.GetChildren()) > 1, true)
	// src, _ := json.MarshalIndent(js.Help, "", "\t")
	// fmt.Printf("DEBUG js.Help: %s\n", src)
	// src, _ = json.MarshalIndent(js.AutoCompleter, "", "\t")
	// fmt.Printf("DEBUG js.AutoCompleter: %s\n", src)
}

func TestPolyfills(t *testing.T) {
	vm := otto.New()
	js := New(vm)
	js.AddExtensions()
	// Check to see if we have a workbook with two sheets
	val, err := js.VM.Eval(`
		(function () {
			if (typeof Number.prototype.parseInt !== "function") {
				console.log("Number.parseInt() missing", typeof Number.parseInt);
				return false;
			}
			if (typeof Number.prototype.parseFloat === undefined) {
				console.log("Number.parseFloat() missing");
				return false;
			}
			n = new Number;
			i = n.parseInt("3", 10)
			if (i !== 3) {
				console.log('n.parseInt("3", 10) failed, i returned was', i);
				return false;
			}
			f = n.parseFloat("3.14");
			if (f !== 3.14) {
				console.log('n.parseFloat("3.14") failed, f returned was', f);
			}
			return true;
		}());
	`)
	if err != nil {
		t.Errorf("xlsx.read() failed, %s", err)
	} else {
		testResult, err := val.ToBoolean()
		if err != nil {
			t.Errorf("xlsx.read(), can't read sheet count, %s", err)
		}
		if testResult == false {
			t.FailNow()
		}
	}
}

func TestWorkbookRead(t *testing.T) {
	vm := otto.New()
	js := New(vm)
	js.AddExtensions()
	// Check to see if we have a workbook with two sheets
	val, err := js.VM.Eval(`
		(function () {
			var wk = xlsx.read("testdata/Workbook1.xlsx");
			if (typeof wk !== "object") {
				console.log("Workbook type of object, ", typeof wk);
				return false;
			}
			var keys = Object.keys(wk);
			if (typeof keys !== "object" && typeof keys !== "array") {
				console.log("keys type of object or array, ", typeof keys);
				return false;
			}
			if (keys.length !== 2) {
				console.log("Expected two worksheets in testdata/Workbook1.xlsx, ", keys.length);
				return false;
			}
			if (keys[0] !== "Sheet1") {
				console.log("Expected sheet zero to be named Sheet1 ", keys[0]);
				return false;
			}
			if (keys[1] !== "Sheet2") {
				console.log("Expected sheet one to be named Sheet2 ", keys[1]);
				return false;
			}
			return true;
		}());
	`)
	if err != nil {
		t.Errorf("xlsx.read() failed, %s", err)
	} else {
		testResult, err := val.ToBoolean()
		if err != nil {
			t.Errorf("xlsx.read(), can't read sheet count, %s", err)
		}
		if testResult == false {
			t.FailNow()
		}
	}
}

func TestWorkbookWrite(t *testing.T) {
	vm := otto.New()
	js := New(vm)
	js.AddExtensions()
	// Check to see if we have a workbook with two sheets
	val, err := js.VM.Eval(`
		(function () {
			var wk = xlsx.read("testdata/Workbook1.xlsx");
			var result = false;
			result = xlsx.write("testout.xlsx", wk);
			if (result !== true) {
				console.log("Could not write ", JSON.Stringify(wk), JSON.Stringify(result, null, "  "));
			}
			return result;
		}());
	`)
	if err != nil {
		t.Errorf("xlsx.read() failed, %s", err)
	} else {
		testResult, err := val.ToBoolean()
		if err != nil {
			t.Errorf("xlsx.read(), can't read sheet count, %s", err)
		}
		if testResult == false {
			t.FailNow()
		}
	}
}
