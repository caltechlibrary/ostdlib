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
