/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright The KubeVirt Authors.
 *
 */

package template

import (
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/utils/ptr"
)

var _ = Describe("visitNonNilValue", func() {
	It("should handle nil pointer", func() {
		var ptr *string
		Expect(visitValue(reflect.ValueOf(ptr), defaultTransformer)).To(Succeed())
	})

	It("should handle nil slice", func() {
		var slice []string
		Expect(visitValue(reflect.ValueOf(slice), defaultTransformer)).To(Succeed())
	})

	It("should handle nil map", func() {
		var m map[string]string
		Expect(visitValue(reflect.ValueOf(m), defaultTransformer)).To(Succeed())
	})

	It("should handle nil interface", func() {
		var i interface{}
		Expect(visitValue(reflect.ValueOf(i), defaultTransformer)).To(Succeed())
	})
})

var _ = Describe("visitValue", func() {
	It("should transform string in pointer", func() {
		ptr := ptr.To("original")
		err := visitNonNilValue(reflect.ValueOf(ptr), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(*ptr).To(Equal("original-mod"))
	})

	It("should transform strings in slice", func() {
		slice := []string{"a", "b", "c"}
		err := visitNonNilValue(reflect.ValueOf(slice), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(slice).To(Equal([]string{"a-mod", "b-mod", "c-mod"}))
	})

	It("should transform strings in array", func() {
		arr := [3]string{"a", "b", "c"}
		err := visitNonNilValue(reflect.ValueOf(&arr).Elem(), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(arr).To(Equal([3]string{"a-mod", "b-mod", "c-mod"}))
	})

	It("should transform strings in struct", func() {
		type TestStruct struct {
			Field1 string
			Field2 string
		}
		obj := TestStruct{Field1: "a", Field2: "b"}
		err := visitNonNilValue(reflect.ValueOf(&obj).Elem(), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(obj.Field1).To(Equal("a-mod"))
		Expect(obj.Field2).To(Equal("b-mod"))
	})

	It("should transform strings in nested struct", func() {
		type Inner struct {
			Value string
		}
		type Outer struct {
			Inner Inner
			Name  string
		}
		obj := Outer{Inner: Inner{Value: "inner"}, Name: "outer"}
		err := visitNonNilValue(reflect.ValueOf(&obj).Elem(), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(obj.Inner.Value).To(Equal("inner-mod"))
		Expect(obj.Name).To(Equal("outer-mod"))
	})

	It("should transform map keys and values", func() {
		m := map[string]string{"key1": "val1", "key2": "val2"}
		err := visitNonNilValue(reflect.ValueOf(&m).Elem(), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(m).To(HaveLen(2))
		Expect(m).To(HaveKeyWithValue("key1-mod", "val1-mod"))
		Expect(m).To(HaveKeyWithValue("key2-mod", "val2-mod"))
	})

	It("should ignore non-string fields", func() {
		type TestStruct struct {
			IntField    int
			BoolField   bool
			FloatField  float64
			StringField string
		}
		obj := TestStruct{IntField: 42, BoolField: true, FloatField: 3.14, StringField: "test"}
		err := visitNonNilValue(reflect.ValueOf(&obj).Elem(), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(obj.IntField).To(Equal(42))
		Expect(obj.BoolField).To(BeTrue())
		Expect(obj.FloatField).To(Equal(3.14))
		Expect(obj.StringField).To(Equal("test-mod"))
	})

	It("should return error when transformer returns non-string for string field", func() {
		err := visitNonNilValue(reflect.ValueOf(ptr.To("test")).Elem(), func(s string) (string, bool, error) {
			return "5", false, nil
		})
		Expect(err).To(MatchError("attempted to set String field to non-string value '5'"))
	})

	It("should handle deeply nested structures", func() {
		type Level struct {
			Value string
			Next  *Level
		}

		// Create 10-level deep structure
		leaf := &Level{Value: "leaf"}
		current := leaf
		for i := range 10 {
			current = &Level{Value: fmt.Sprintf("level%d", i), Next: current}
		}

		Expect(visitNonNilValue(reflect.ValueOf(current), defaultTransformer)).To(Succeed())

		// Verify all levels were transformed
		for i := 9; i >= 0; i-- {
			Expect(current.Value).To(Equal(fmt.Sprintf("level%d-mod", i)))
			current = current.Next
		}

		Expect(current.Value).To(Equal("leaf-mod"))
	})
})

var _ = Describe("visitSliceArray", func() {
	It("should handle slice of structs", func() {
		type TestStruct struct {
			Name string
		}
		slice := []TestStruct{
			{Name: "first"},
			{Name: "second"},
		}
		err := visitSliceArray(reflect.ValueOf(slice), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(slice[0].Name).To(Equal("first-mod"))
		Expect(slice[1].Name).To(Equal("second-mod"))
	})

	It("should handle slice of pointers", func() {
		slice := []*string{ptr.To("first"), ptr.To("second")}
		err := visitSliceArray(reflect.ValueOf(slice), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(*slice[0]).To(Equal("first-mod"))
		Expect(*slice[1]).To(Equal("second-mod"))
	})

	It("should handle slice of any", func() {
		slice := []any{"string", 42, true}
		err := visitSliceArray(reflect.ValueOf(slice), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(slice[0]).To(Equal("string-mod"))
		Expect(slice[1]).To(Equal(42))
		Expect(slice[2]).To(BeTrue())
	})

	It("should handle slice with mixed nil and non-nil values", func() {
		slice := []any{"string", nil, "another"}
		err := visitSliceArray(reflect.ValueOf(slice), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(slice[0]).To(Equal("string-mod"))
		Expect(slice[1]).To(BeNil())
		Expect(slice[2]).To(Equal("another-mod"))
	})
})

var _ = Describe("visitStruct", func() {
	It("should handle struct with pointer fields", func() {
		type TestStruct struct {
			Ptr *string
		}
		obj := TestStruct{Ptr: ptr.To("test")}
		err := visitStruct(reflect.ValueOf(obj), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(*obj.Ptr).To(Equal("test-mod"))
	})

	It("should handle struct with nil pointer fields", func() {
		type TestStruct struct {
			Ptr *string
		}
		obj := TestStruct{Ptr: nil}
		err := visitStruct(reflect.ValueOf(obj), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(obj.Ptr).To(BeNil())
	})

	It("should handle empty struct", func() {
		type EmptyStruct struct{}
		obj := EmptyStruct{}
		err := visitStruct(reflect.ValueOf(obj), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should skip unexported fields in struct", func() {
		type TestStruct struct {
			Exported   string
			unexported string
		}
		obj := TestStruct{Exported: "public", unexported: "private"}
		err := visitStruct(reflect.ValueOf(&obj).Elem(), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(obj.Exported).To(Equal("public-mod"))
		Expect(obj.unexported).To(Equal("private")) // Should remain unchanged
	})
})

var _ = Describe("visitMap", func() {
	It("should handle map with string keys", func() {
		m := map[string]any{
			"key1": "string-value",
			"key2": 42,
			"key3": true,
		}
		err := visitMap(reflect.ValueOf(m), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(m).To(HaveLen(3))
		Expect(m).To(HaveKeyWithValue("key1-mod", "string-value-mod"))
		Expect(m).To(HaveKeyWithValue("key2-mod", 42))
		Expect(m).To(HaveKeyWithValue("key3-mod", true))
	})

	It("should handle map with non-string keys", func() {
		m := map[int]any{1: "one", 2: 42, 3: true}
		err := visitMap(reflect.ValueOf(m), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(m).To(HaveLen(3))
		Expect(m).To(HaveKeyWithValue(1, "one-mod"))
		Expect(m).To(HaveKeyWithValue(2, 42))
		Expect(m).To(HaveKeyWithValue(3, true))
	})

	It("should handle nested maps with string keys", func() {
		m := map[string]map[string]string{
			"outer": {
				"inner": "value",
			},
		}
		err := visitMap(reflect.ValueOf(m), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(m).To(HaveLen(1))
		Expect(m).To(HaveKey("outer-mod"))
		Expect(m["outer-mod"]).To(HaveLen(1))
		Expect(m["outer-mod"]).To(HaveKeyWithValue("inner-mod", "value-mod"))
	})

	It("should handle nested maps with non-string keys", func() {
		m := map[string]map[int]string{
			"outer": {
				1: "value",
			},
		}
		err := visitMap(reflect.ValueOf(m), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(m).To(HaveLen(1))
		Expect(m).To(HaveKey("outer-mod"))
		Expect(m["outer-mod"]).To(HaveLen(1))
		Expect(m["outer-mod"]).To(HaveKeyWithValue(1, "value-mod"))
	})

	It("should handle map key collision during transformation", func() {
		m := map[string]string{
			"key":     "original",
			"key-mod": "should-not-be-lost",
		}
		err := visitMap(reflect.ValueOf(m), defaultTransformer)
		Expect(err).ToNot(HaveOccurred())
		Expect(m).To(HaveLen(2))
		Expect(m).To(HaveKeyWithValue("key-mod", "original-mod"))
		Expect(m).To(HaveKeyWithValue("key-mod-mod", "should-not-be-lost-mod"))
	})
})

var _ = Describe("visitUnsettableValues", func() {
	It("should handle string to string transformation", func() {
		val, err := visitUnsettableValues(
			reflect.TypeOf(""),
			reflect.ValueOf("test"),
			defaultTransformer,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(val.String()).To(Equal("test-mod"))
	})

	It("should handle string to float64 transformation", func() {
		val, err := visitUnsettableValues(
			reflect.TypeOf(float64(0)),
			reflect.ValueOf("42"),
			func(s string) (string, bool, error) { return "42", false, nil },
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(val.Interface()).To(Equal(float64(42))) // JSON unmarshals numbers as float64
	})

	It("should handle string to bool transformation", func() {
		const trueStr = "true"
		val, err := visitUnsettableValues(
			reflect.TypeOf(false),
			reflect.ValueOf(trueStr),
			func(s string) (string, bool, error) { return trueStr, false, nil },
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(val.Interface()).To(BeTrue())
	})

	It("should fallback to string when JSON unmarshal fails", func() {
		val, err := visitUnsettableValues(
			reflect.TypeOf(""),
			reflect.ValueOf("not-json"),
			func(s string) (string, bool, error) { return "not-json", false, nil },
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(val.Interface()).To(Equal("not-json"))
	})

	It("should return error when type is not assignable", func() {
		val, err := visitUnsettableValues(
			reflect.TypeOf(0),
			reflect.ValueOf("true"),
			func(s string) (string, bool, error) { return "true", false, nil },
		)
		Expect(err).To(MatchError("substituted value type bool is not assignable to target type int"))
		Expect(val.IsValid()).To(BeFalse())
	})

	It("should handle nil data from JSON unmarshal", func() {
		val, err := visitUnsettableValues(
			reflect.TypeOf(""),
			reflect.ValueOf("null"),
			func(s string) (string, bool, error) { return "null", false, nil },
		)
		Expect(err).To(MatchError("cannot assign nil value to target type string"))
		Expect(val.IsValid()).To(BeFalse())
	})

	It("should handle any type", func() {
		var iface any = "test"
		val, err := visitUnsettableValues(
			reflect.TypeOf(""),
			reflect.ValueOf(iface),
			defaultTransformer,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(val.String()).To(Equal("test-mod"))
	})

	It("should handle non-string existing value", func() {
		type TestStruct struct {
			Field string
		}
		existing := TestStruct{Field: "test"}
		val, err := visitUnsettableValues(
			reflect.TypeOf(TestStruct{}),
			reflect.ValueOf(existing),
			defaultTransformer,
		)
		Expect(err).ToNot(HaveOccurred())
		result, ok := val.Interface().(TestStruct)
		Expect(ok).To(BeTrue())
		Expect(result.Field).To(Equal("test-mod"))
	})
})

func defaultTransformer(in string) (out string, asString bool, err error) {
	return in + "-mod", true, nil
}
