package common

import (
    "bytes"
    "errors"
    "fmt"
    "regexp"
)

type ParserOptions struct {
    VariableRegex string
    EscapeRegex   string
    Callbacks     ParserFunctions
}

type ParserFunctions interface {
    UnescapeValue(value string) string
    UnwrapVariable(variableMatch string) string
}

type Parser struct {
    escapeRE  *regexp.Regexp
    callbacks ParserFunctions
    segments  []*parserSegment
}

type parserSegment struct {
    variable string
    searcher *stringSearcher
}

var NO_MATCH error = errors.New("Match not found")

func MakeParser(format string, options *ParserOptions) (*Parser, error) {
    // Compile variable regexp
    varRE, err := regexp.Compile(options.VariableRegex)
    if err != nil {
        return nil, err
    }
    
    escRE, err := regexp.Compile(options.EscapeRegex)
    if err != nil {
        return nil, err
    }
    
    // Split format into variables/separator
    segments := splitSegments(format, varRE)
    
    // Combine var-sep-var-sep-... sequence into []*parserSegment
    // We'll compile (Boyer-Moore) the separators and cache those results since they're
    // probably reused.
    var psegments []*parserSegment
    var variable string
    searchers := make(map[string]*stringSearcher)
    for _, segment := range segments {
        if varRE.MatchString(segment) {
            // Variable
            if variable != "" {
                // Two consecutive variables
                return nil, fmt.Errorf("Two consecutive variables in format: %s, %s", variable, segment)
            }
            
            variable = segment
        } else {
            // Separator
            searcher, ok := searchers[segment]
            if !ok {
                searcher = compileSearcher(segment)
                searchers[segment] = searcher
            }
            
            psegments = append(psegments, &parserSegment{
                variable: options.Callbacks.UnwrapVariable(variable),
                searcher: searcher,
            })
            
            variable = ""
        }
    }
    
    if variable != "" {
        // If we ended with a variable, then add a parser segment for it.
        psegments = append(psegments, &parserSegment{
            variable: options.Callbacks.UnwrapVariable(variable),
            searcher: nil,
        })
    }
    
    return &Parser{
        escapeRE:  escRE,
        segments:  psegments,
        callbacks: options.Callbacks,
    }, nil
}

func (parser *Parser) Parse(line string) (map[string]string, error) {
    // First, find all of the escape sequences in the input so we can skip over them
    // when processing the line.
    escapes := parser.escapeRE.FindAllStringIndex(line, -1)
    //fmt.Printf("Escapes: %q\n", escapes)
    
    result := make(map[string]string)
    ptr := 0
    for _, segment := range(parser.segments) {
/*
        pat := ""
        if segment.searcher != nil {
            pat = segment.searcher.pattern
        }
        
        fmt.Printf("Processing segment, var=%s, sep=%s\n", segment.variable, pat)
*/
        if segment.variable == "" {
            // Look for a delimiter at the beginning, don't read into a variable
            _, eidx, escidx, err := segment.searcher.Search(line, ptr, escapes)
            if err != nil {
                return nil, err
            }
            
            ptr = eidx
            escapes = escapes[escidx:]
        } else if segment.searcher == nil {
            // Read the rest of the line into a variable
            value := line[ptr:]
            if len(escapes) > 0 {
                value = parser.unescape(value, ptr, escapes)
            }
            
            result[segment.variable] = value
            ptr = len(line)
        } else {
            // Find separator, 
            idx, eidx, escidx, err := segment.searcher.Search(line, ptr, escapes)
            if err != nil {
                return nil, err
            }
            
            // Unescape the value only if we skipped over any escapes
            value := line[ptr:idx]
            if escidx > 0 {
                value = parser.unescape(value, ptr, escapes[0:escidx])
                escapes = escapes[escidx:]
            }
            
            result[segment.variable] = value
            ptr = eidx
        }
    }
    
    return result, nil
}

func (parser *Parser) unescape(match string, offset int, escapes [][]int) string {
    var buf bytes.Buffer
    last := 0
    
    for _, esc := range escapes {
        // Get escape relative to offset
        escStart := esc[0] - offset
        escEnd := esc[1] - offset
        
        // Write last-escape start into buffer
        buf.WriteString(match[last:escStart])
        
        // Unescape into buffer
        buf.WriteString(parser.callbacks.UnescapeValue(match[escStart:escEnd]))
        
        // Advance last pointer
        last = escEnd
    }
    
    // Write remainder of string
    buf.WriteString(match[last:])
    
    return buf.String()
}

func splitSegments(format string, varRE *regexp.Regexp) []string {
    var segments []string
    
    for len(format) > 0 {
        loc := varRE.FindStringIndex(format)
        if loc == nil {
            segments = append(segments, format)
            break
        } else {
            if loc[0] > 0 {
                segments = append(segments, format[:loc[0]])
            }
            
            segments = append(segments, format[loc[0]:loc[1]])
            format = format[loc[1]:]
        }
    }
    
    return segments
}

type stringSearcher struct {
    pattern      string
    badChars     [256]int
    goodSuffixes []int
}

func compileSearcher(pattern string) *stringSearcher {
    result := &stringSearcher{pattern: pattern}
    length := len(pattern)
    last := length - 1
    
    // Bad character rule
    for i := 0; i < 256; i++ {
        result.badChars[i] = length
    }
    for i := 0; i < length; i++ {
        result.badChars[pattern[i]] = last - i
    }
    
    // Good suffix rule - http://www-igm.univ-mlv.fr/~lecroq/string/node14.html
    suffixes := make([]int, length)
    
    good := last
    f := last - 1
    suffixes[last] = length
    
    for i := last - 1; i >= 0; i-- {
        if i > good && suffixes[i + last - f] < i - good {
            suffixes[i] = suffixes[i + last - f]
        } else {
            if i < good {
                good = i
            }
            f = i
            for good >= 0 && pattern[good] == pattern[good + last - f] { good-- }
            suffixes[i] = f - good
        }
    }
    
    result.goodSuffixes = make([]int, length)
    for i := 0; i < length; i++ {
        result.goodSuffixes[i] = length
    }
    
    j := 0
    for i := last; i >= 0; i-- {
        if suffixes[i] == i + 1 {
            for ; j < last - i; j++ {
                if result.goodSuffixes[j] == length {
                    result.goodSuffixes[j] = last - i
                }
            }
        }
    }
    
    for i := 0; i < last; i++ {
        result.goodSuffixes[last - suffixes[i]] = last - i
    }
    
    return result
}

func (search *stringSearcher) Search(line string, start int, escapes [][]int) (int, int, int, error) {
    escidx := 0
    
    for i := start; i <= len(line) - len(search.pattern); {
        j := len(search.pattern) - 1
        //debugStep(line, i, j)
        
        // Skip over escapes we've already passed
        for escidx < len(escapes) && escapes[escidx][1] <= i { escidx++ }
        
        // Skip i over the next escape if we're in the middle of it
        if escidx < len(escapes) && escapes[escidx][0] <= (j + i) {
            i = escapes[escidx][1]
            //debugOut("Skipping over escape")
            continue
        }
        
        // Perform check
        for j >= 0 && search.pattern[j] == line[i + j] { j-- }
        if j < 0 {
            // Matched
            //debugOut("Found!")
            return i, i + len(search.pattern), escidx, nil
        }
        
        // No match
        bc := search.badChars[line[i + j]] - len(search.pattern) + 1 + j
        gs := search.goodSuffixes[j]
        
        //debugOut("bc=%d, gs=%d", bc, gs)
        
        if bc > gs {
            i += bc
        } else {
            i += gs
        }
    }
    
    return 0, 0, 0, NO_MATCH
}

func debugStep(line string, i, j int) {
    fmt.Printf("\t%s\n\t", line)
    for t := 0; t < i; t++ {
        fmt.Printf(" ")
    }
    
    for t := 0; t <= j; t++ {
        fmt.Printf("^")
    }
    
    fmt.Println("")
}

func debugOut(format string, args... interface{}) {
    fmt.Printf(format + "\n", args...)
}
