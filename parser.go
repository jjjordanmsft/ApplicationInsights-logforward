
package main

import (
    "bytes"
    "fmt"
    "net/url"
    "regexp"
    "strconv"
    "strings"
    "time"
    
    "github.com/jjjordanmsft/ApplicationInsights-Go/appinsights"
)

const (
    defaultFormat = "$remote_addr - $remote_user [$time_local] \"$request\" $status $body_bytes_sent \"$http_referer\" \"$http_user_agent\" \"$http_x_forwarded_for\""
)

var (
    varRE = regexp.MustCompile("\\$[a-zA-Z0-9_]+")
)

type LogParser struct {
    fmtRE	*regexp.Regexp
    jsonEscape	bool
}

func MakeLogParser(logFormat string, jsonEscape bool) (*LogParser, error) {
    regexExpr := makeLogRegexp(logFormat, jsonEscape)
    regex, err := regexp.Compile(regexExpr)
    if err != nil {
        return nil, err
    }
    
    return &LogParser{fmtRE: regex, jsonEscape: jsonEscape}, nil
}

func makeLogRegexp(format string, jsonEscape bool) string {
    var result bytes.Buffer
    
    for len(format) > 0 {
        loc := varRE.FindStringIndex(format)
        if loc == nil {
            // Write out rest of string and finish
            result.WriteString(regexp.QuoteMeta(format))
            break
        } else {
            if (loc[0] > 0) {
                // Write line through variable
                result.WriteString(regexp.QuoteMeta(format[0:loc[0]]))
            }
            
            // Grab the variable name
            varname := format[loc[0]+1:loc[1]]
            
            if (loc[1] < len(format)) {
                // If there are no more variables, then we can be a little more
                // liberal with the final capture
                if (strings.IndexByte(format[loc[1]:len(format)], '$') < 0) {
                    // Don't need to use lookahead character
                    fmt.Fprintf(&result, "(?P<%s>.*)", varname)
                } else {
                    fmt.Fprintf(&result, "(?P<%s>[^%s]*)", varname, regexp.QuoteMeta(format[loc[1]:loc[1]+1]))
                }
            } else {
                // To end-of-line
                fmt.Fprintf(&result, "(?P<%s>.*)", varname)
            }
            
            // Cut out variable
            format = format[loc[1]:len(format)]
        }
    }
    
    return result.String()
}

func (parser *LogParser) parseLogLine(line string) (map[string]string, error) {
    matches := parser.fmtRE.FindStringSubmatch(line)
    if len(matches) < 1 {
        return nil, fmt.Errorf("Line doesn't match format")
    }
    
    subnames := parser.fmtRE.SubexpNames()
    result := make(map[string]string)
    
    for i := 1; i < len(subnames); i++ {
        result[subnames[i]] = matches[i]
    }
    
    return result, nil
}

func (parser *LogParser) CreateTelemetry(line string) (*appinsights.RequestTelemetry, error) {
    log, err := parser.parseLogLine(line)
    if err != nil {
        return nil, err
    }
    
    name, err := parseName(log)
    if err != nil {
        return nil, fmt.Errorf("Error parsing request name: %s", err.Error())
    }
    
    timestamp, err := parseTimestamp(log)
    if err != nil {
        return nil, fmt.Errorf("Error parsing timestamp: %s", err.Error())
    }
    
    duration, err := parseDuration(log)
    if err != nil {
        return nil, fmt.Errorf("Error parsing duration: %s", err.Error())
    }
    
    responseCode, err := parseResponseCode(log)
    if err != nil {
        return nil, fmt.Errorf("Error parsing response code: %s", err.Error())
    }
    
    success, err := parseSuccess(log)
    if err != nil {
        return nil, err
    }
    
    telem := appinsights.NewRequestTelemetry(name, timestamp, duration, responseCode, success)
    
    return telem, nil
}

func parseName(log map[string]string) (string, error) {
    if val, ok := log["request"]; ok {
        return val, nil
    }
    
    return "", fmt.Errorf("No key exists to get request name")
}

func parseTimestamp(log map[string]string) (time.Time, error) {
    if val, ok := log["time_local"]; ok {
        if tm, err := time.Parse("02/Jan/2006:15:04:05 -0700", val); err == nil {
            return tm, nil
        }
    }
    
    if val, ok := log["time_iso8601"]; ok {
        if tm, err := time.Parse(time.RFC3339, val); err == nil {
            return tm, nil
        }
    }
    
    return time.Time{}, fmt.Errorf("No time specified, or in the wrong format")
}

func parseDuration(log map[string]string) (time.Duration, error) {
    if val, ok := log["request_time"]; ok {
        duration, err := strconv.ParseFloat(val, 64)
        if err != nil {
            return 0, fmt.Errorf("Error parsing request duration: %s", err.Error())
        }
        
        return time.Duration(duration * float64(time.Second)), nil
    }
    
    // Not a big deal, and not common
    return 0, nil
}

func parseResponseCode(log map[string]string) (string, error) {
    if val, ok := log["status"]; ok {
        return val, nil
    }
    
    return "", fmt.Errorf("No response code available")
}

func parseSuccess(log map[string]string) (bool, error) {
    if code, err := parseResponseCode(log); err == nil {
        if n, err := strconv.Atoi(code); err == nil {
            return n < 400, nil
        } else {
            return false, fmt.Errorf("Error parsing response code: %s", err.Error())
        }
    }
    
    // Default
    return false, fmt.Errorf("No response code available")
}

func parseUrl(log map[string]string) (string, error) {
    // Ideal input
    if val, ok := log["request_uri"]; ok {
        return val, nil
    }
    
    // $uri can change around depending on config, this may be OK
    if uri, ok := log["uri"]; ok {
        return uri, nil
    }
    
    // Have each component piece
    scheme, schemeok := log["schema"]
    path, pathok := log["request_path"]
    vhost, vhostok := log["host"]
    request, requestok := log["request"]
    
    // Try to piece together as much as we can
    var reqpath *url.URL
    
    if pathok {
        reqpath, _ = url.Parse(path)
    }
    
    if requestok {
        parts := strings.Split(request, " ")
        if len(parts) == 3 {
            reqpath, _ = url.Parse(parts[1])
        }
    }
    
    if reqpath == nil {
        return "", fmt.Errorf("Can't get request URI from log line")
    }
    
    if reqpath.Scheme == "" && schemeok {
        reqpath.Scheme = scheme
    }
    
    if reqpath.Host == "" && vhostok {
        reqpath.Host = vhost
    }
    
    return reqpath.String(), nil
}
