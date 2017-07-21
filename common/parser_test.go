package common

import (
    "bytes"
    "fmt"
    "testing"
)

type testParserCallbacks struct {}

func testUnwrapVariable(variable string) string {
    return variable
}

func makeOptions() *ParserOptions {
    return &ParserOptions{
        VariableRegex: `\$\{[^}]+\}`,
        EscapeRegex: UnescapeCommonPattern,
        Unescape: UnescapeCommon,
        UnwrapVariable: testUnwrapVariable,
    }
}

func TestBoyerMoore(t *testing.T) {
    parser, _ := NewParser("${front} - ${first} - ${second}", makeOptions())
    Show(parser.Parse("1 - 2 -3-  - 3"))
    
    parser, _ = NewParser(`"${first}" "${second}" "${third}"`, makeOptions())
    Show(parser.Parse(`"this is" "some \" " "\"Text!\""`))
    Show(parser.Parse(`"this is" "some " "Text!"`))
}

func Show(val map[string]string, err error) {
    if err != nil {
        fmt.Printf("Error: %s\n", err.Error())
    } else {
        var buf bytes.Buffer
        for k, v := range(val) {
            fmt.Fprintf(&buf, "%s: %s\n", k, v)
        }
        fmt.Println(buf.String())
    }
}
