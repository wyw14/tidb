// Copyright 2019 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeEscapedUnicode(t *testing.T) {
	testCases := []struct {
		input            string
		expectedResult   string
		size             int
		inSurrogateRange bool
		expectedValid    bool
	}{
		{"597d", "好\x00", 3, false, true},
		{"fffd", "�\x00", 3, false, true},
		{"D83DDE0A", "😊", 4, false, true},
		{"D83D", "", 0, true, false},
		{"D83D11", "", 0, false, false},
		{"ZZZZ", "", 0, false, false},
		{"D83DDE0A597d", "", 0, false, false},
	}

	for _, tc := range testCases {
		result, size, inSurrogateRange, err := decodeOneEscapedUnicode([]byte(tc.input))
		require.Equal(t, tc.inSurrogateRange, inSurrogateRange)
		if tc.expectedValid {
			require.NoError(t, err)
			require.Equal(t, tc.expectedResult, string(result[:]))
			require.Equal(t, tc.size, size)
		} else {
			require.Error(t, err)
		}
	}
}

func TestUnquoteJSONString(t *testing.T) {
	var testCases = []struct {
		input          string
		expectedResult string
		expectedValid  bool
	}{
		{"\\b", "\b", true},
		{"\\f", "\f", true},
		{"\\n", "\n", true},
		{"\\r", "\r", true},
		{"\\t", "\t", true},
		{"\\\\", "\\", true},
		{"\\u597d", "好", true},
		{"0\\u597d0", "0好0", true},
		{"\\a", "a", true},
		{"[", "[", true},
		{"\\ud83e\\udd21", "🤡", true},
		{"\\ufffd", "�", true},
		// invalid input
		{"\\", "", false},
		{"\\u59", "", false},
	}

	for _, tc := range testCases {
		result, err := unquoteJSONString(tc.input)
		if tc.expectedValid {
			require.NoError(t, err)
			require.Equal(t, tc.expectedResult, result)
		} else {
			require.Error(t, err)
		}
	}
}

func BenchmarkDecodeEscapedUnicode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		in := "597d"
		_, _, _, _ = decodeOneEscapedUnicode([]byte(in))
	}
}

func BenchmarkMergePatchBinary(b *testing.B) {
	valueA, _ := ParseBinaryJSONFromString(`{"title":"Goodbye!","author":{"givenName":"John","familyName":"Doe"},"tags":["example","sample"],"content":"This will be unchanged"}`)
	valueB, _ := ParseBinaryJSONFromString(`{"title":"Hello!","phoneNumber":"+01-123-456-7890","author":{"familyName":null},"tags":["example"]}`)
	for i := 0; i < b.N; i++ {
		_, _ = MergePatchBinaryJSON([]*BinaryJSON{&valueA, &valueB})
	}
}

func BenchmarkMergeBinary(b *testing.B) {
	valueA, _ := ParseBinaryJSONFromString(`{"title":"Goodbye!","author":{"givenName":"John","familyName":"Doe"},"tags":["example","sample"],"content":"This will be unchanged"}`)
	valueB, _ := ParseBinaryJSONFromString(`{"title":"Hello!","phoneNumber":"+01-123-456-7890","author":{"familyName":null},"tags":["example"]}`)
	for i := 0; i < b.N; i++ {
		_ = MergeBinaryJSON([]BinaryJSON{valueA, valueB})
	}
}

func TestBinaryCompare(t *testing.T) {
	tests := []struct {
		left   BinaryJSON
		right  BinaryJSON
		result int
	}{
		{
			CreateBinaryJSON("a"),
			CreateBinaryJSON("b"),
			-1,
		},
		{
			CreateBinaryJSON(Opaque{
				TypeCode: 0,
				Buf:      []byte{0, 1, 2, 3},
			}),
			CreateBinaryJSON(Opaque{
				TypeCode: 0,
				Buf:      []byte{0, 1, 2},
			}),
			1,
		},
		{
			CreateBinaryJSON(Opaque{
				TypeCode: 0,
				Buf:      []byte{0, 1, 2, 3},
			}),
			CreateBinaryJSON(Opaque{
				TypeCode: 0,
				Buf:      []byte{0, 2, 1},
			}),
			-1,
		},
		{
			CreateBinaryJSON("test"),
			CreateBinaryJSON(Opaque{
				TypeCode: 0,
				Buf:      []byte{0, 2, 1},
			}),
			-1,
		},
	}

	compareMsg := map[int]string{
		1:  "greater than",
		0:  "equal with",
		-1: "smaller than",
	}

	for _, test := range tests {
		require.Equal(t, test.result, CompareBinaryJSON(test.left, test.right), "%s should be %s %s", test.left.String(), compareMsg[test.result], test.right.String())
	}
}

func TestEscapeJSONPointerToken(t *testing.T) {
	// Examples from RFC 6901 section 5.
	cases := []struct {
		token    string
		expected string
	}{
		{"", ""},
		{" ", " "},
		{"abc", "abc"},
		{"a/b", "a~1b"},
		{"c%d", "c%d"},
		{"e^f", "e^f"},
		{"m~n", "m~0n"},
		{"/~", "~1~0"},
		{"~1", "~01"},
		{"~/", "~0~1"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.expected, escapeJSONPointerToken(tc.token))
	}
}

func TestDiffBinaryJSON(t *testing.T) {
	tests := []struct {
		oldDoc   string
		newDoc   string
		expected string
	}{
		// Equal documents yield an empty array.
		{`1`, `1`, `[]`},
		{`1.5`, `1.5`, `[]`},
		{`{}`, `{}`, `[]`},
		{`[1, {"a": 2}, null]`, `[1, {"a": 2}, null]`, `[]`},
		// Numeric equality follows the existing JSON comparison semantics:
		// integer 1 equals double 1.0.
		{`1`, `1.0`, `[]`},
		{`[1]`, `[1.0]`, `[]`},
		// Root scalars are replaced as a whole with an empty root path.
		{`1`, `2`, `[{"old": 1, "new": 2, "op": "replace", "path": ""}]`},
		{`"a"`, `"b"`, `[{"old": "a", "new": "b", "op": "replace", "path": ""}]`},
		{`false`, `null`, `[{"old": false, "new": null, "op": "replace", "path": ""}]`},
		// A type change at root is a whole replace.
		{`{"a": 1}`, `[1]`, `[{"old": {"a": 1}, "new": [1], "op": "replace", "path": ""}]`},
		// Arrays at root are compared as a whole.
		{`[1, 2]`, `[1, 3]`, `[{"old": [1, 2], "new": [1, 3], "op": "replace", "path": ""}]`},
		// Object member add/remove/replace.
		{
			`{"a": 1, "b": 2}`,
			`{"b": 2, "c": 3}`,
			`[
				{"op": "remove", "path": "/a", "old": 1},
				{"op": "add", "path": "/c", "new": 3}
			]`,
		},
		// Recursion into common objects; equal members emit nothing.
		{
			`{"a": {"x": 1}, "b": [1, 2]}`,
			`{"a": {"x": 1, "y": 2}, "b": [1, 2]}`,
			`[{"op": "add", "path": "/a/y", "new": 2}]`,
		},
		// Adding or removing a whole subtree emits a single entry.
		{
			`{"a": {"x": [1, 2]}}`,
			`{}`,
			`[{"op": "remove", "path": "/a", "old": {"x": [1, 2]}}]`,
		},
		{
			`{}`,
			`{"a": {"x": [1, 2]}}`,
			`[{"op": "add", "path": "/a", "new": {"x": [1, 2]}}]`,
		},
		// An array inside an object is replaced as a whole.
		{
			`{"a": [1, 2]}`,
			`{"a": [1, 3]}`,
			`[{"op": "replace", "path": "/a", "old": [1, 2], "new": [1, 3]}]`,
		},
		// When a member changes between object and scalar, it is one replace
		// instead of per-child add/remove entries.
		{
			`{"a": {"x": 1}}`,
			`{"a": null}`,
			`[{"op": "replace", "path": "/a", "old": {"x": 1}, "new": null}]`,
		},
		// A present JSON null value is distinct from a missing member.
		{`{"a": null}`, `{}`, `[{"op": "remove", "path": "/a", "old": null}]`},
		{`{}`, `{"a": null}`, `[{"op": "add", "path": "/a", "new": null}]`},
		{
			`{"a": 1}`,
			`{"a": null}`,
			`[{"op": "replace", "path": "/a", "old": 1, "new": null}]`,
		},
		// add carries no "old" and remove carries no "new".
		{
			`{"a": 1, "b": 2}`,
			`{"b": 3, "c": 4}`,
			`[
				{"op": "remove", "path": "/a", "old": 1},
				{"op": "replace", "path": "/b", "old": 2, "new": 3},
				{"op": "add", "path": "/c", "new": 4}
			]`,
		},
		// Entries are sorted by the byte order of path.
		{
			`{"z": 1, "a": 2, "m": 3}`,
			`{}`,
			`[
				{"op": "remove", "path": "/a", "old": 2},
				{"op": "remove", "path": "/m", "old": 3},
				{"op": "remove", "path": "/z", "old": 1}
			]`,
		},
		// '~' and '/' in object keys are escaped as JSON Pointer tokens.
		{
			`{"a/b": 1, "m~n": 2}`,
			`{"a/b": 3, "m~n": 4}`,
			`[
				{"op": "replace", "path": "/a~1b", "old": 1, "new": 3},
				{"op": "replace", "path": "/m~0n", "old": 2, "new": 4}
			]`,
		},
		// Escaping applies at every path segment, and an empty key maps to "/".
		{
			`{"": 1, "x": {"a/b": 1}}`,
			`{"": 2, "x": {"a/b": 2}}`,
			`[
				{"op": "replace", "path": "/", "old": 1, "new": 2},
				{"op": "replace", "path": "/x/a~1b", "old": 1, "new": 2}
			]`,
		},
	}

	parse := func(t *testing.T, s string) BinaryJSON {
		doc, err := ParseBinaryJSONFromString(s)
		require.NoError(t, err)
		return doc
	}

	for _, tc := range tests {
		oldDoc := parse(t, tc.oldDoc)
		newDoc := parse(t, tc.newDoc)
		expected := parse(t, tc.expected)
		diff := DiffBinaryJSON(oldDoc, newDoc)
		require.Equal(t, expected.String(), diff.String(), "diff(%s, %s)", tc.oldDoc, tc.newDoc)
	}
}
