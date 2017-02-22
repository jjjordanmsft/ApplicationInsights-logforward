
package main

import (
    "bufio"
    "bytes"
    "fmt"
    "io"
    "os"
    "time"
)

type LogReader struct {
    events	chan LogEventMessage
    control	chan LogControlMessage
    closed	bool
}

type LogEventMessage struct {
    data	string
    closed	bool
    err		error
}

type LogControlMessage struct {
    close	bool
    reset	bool
    shutdown	bool
}

func (logReader *LogReader) Reset() {
    logReader.control <- LogControlMessage{reset: true}
}

func (logReader *LogReader) Close() {
    logReader.control <- LogControlMessage{close: true}
}

func (logReader *LogReader) Events() chan LogEventMessage {
    return logReader.events
}

func MakeLogReader(infile string) (*LogReader, error) {
    events := make(chan LogEventMessage)
    control := make(chan LogControlMessage)
    result := &LogReader{events, control, false}
    
    if infile == "-" {
        // Stdin pipe
        err := readStdin(result)
        if err != nil {
            return nil, fmt.Errorf("Error opening stdin: %s", err.Error())
        }
        
        return result, nil
    }
    
    stat, err := os.Stat(infile)
    if err != nil {
        return nil, fmt.Errorf("Error opening input file %s: %s", infile, err.Error())
    }
    
    if (stat.Mode() & os.ModeNamedPipe) != 0 {
        // Named pipe
        err = readFifo(infile, result)
        if err != nil {
            return nil, fmt.Errorf("Error opening input file %s: %s", infile, err.Error())
        }
        
        return result, nil
    }
    
    if stat.Mode().IsRegular() {
        err = readFile(infile, result)
        if err != nil {
            return nil, fmt.Errorf("Error opening input file %s: %s", infile, err.Error())
        }
        
        return result, nil
    }
    
    return nil, fmt.Errorf("%s is not a supported file type", infile)
}

func readStdin(logReader *LogReader) error {
    file := os.Stdin
    scanner := bufio.NewScanner(file)
    
    // Data stream
    go func() {
        for scanner.Scan() {
            fmt.Fprintf(os.Stderr, "Reading a line...\n")
            ln := scanner.Text()
            logReader.events <- LogEventMessage{data: ln}
        }
        
        fmt.Fprintf(os.Stderr, "Broke out of loop\n")
        
        file.Close()
        logReader.closed = true
        logReader.events <- LogEventMessage{closed: true}
        logReader.control <- LogControlMessage{shutdown: true}
    }()
    
    // Control stream
    go func() {
        for {
            select {
            case ctl := <- logReader.control:
                if ctl.close {
                    fmt.Fprintf(os.Stderr, "Got close signal\n")
                    err := file.Close()
                    if err != nil {
                        panic(err)
                    }
                } else if ctl.shutdown {
                    return
                }
            }
        }
    }()
    
    return nil
}

func readFifo(infile string, logReader *LogReader) error {
    file, err := os.OpenFile(infile, os.O_RDONLY, 0)
    if err != nil {
        return err
    }
    
    logReader.closed = false
    events := make(chan LogEventMessage)
    
    // Data stream
    go func() {
        buf := make([]byte, 2048)
        var sbuf bytes.Buffer
        
        for {
            n, err := file.Read(buf)
            if err == io.EOF || n == 0 {
                break
            }
            
            if err != nil {
                events <- LogEventMessage{err: fmt.Errorf("Error while reading %s: %s", infile, err.Error()), closed: true}
                break
            }
            
            st := 0
            for st < n {
                idx := bytes.IndexByte(buf[st:n], '\n')
                if idx < 0 {
                    sbuf.Write(buf[st:n])
                    break
                } else {
                    sbuf.Write(buf[st:st + idx])
                    if sbuf.Len() > 0 {
                        events <- LogEventMessage{data: sbuf.String()}
                        sbuf.Reset()
                    }
                    st += idx + 1
                }
            }
        }
/*        
        scanner := bufio.NewScanner(file)
        for scanner.Scan() {
            ln := scanner.Text()
            events <- LogEventMessage{data: ln}
        }
*/
        fmt.Fprintf(os.Stderr, "Broke out of FIFO loop\n")
        
        logReader.closed = true
        events <- LogEventMessage{closed: true}
        logReader.control <- LogControlMessage{shutdown: true}
    }()
    
    fmt.Fprintln(os.Stderr, "Starting fifo loop")
    
    // Control stream
    go func() {
        for {
            select {
            case event := <- events:
                // Forward event
                logReader.events <- event
            case ctl := <- logReader.control:
                if ctl.close {
                    file.Close()
                } else if ctl.reset {
                    fmt.Fprintln(os.Stderr, "Received reset signal")
                    file.Close()
                    
                    // Wait until it shuts down
waitloop:           for {
                        select {
                        case event := <- events:
                            fmt.Fprintln(os.Stderr, "Got event while waiting for shutdown")
                            if event.data != "" {
                                logReader.events <- event
                            }
                        
                        case ctl := <- logReader.control:
                            fmt.Fprintln(os.Stderr, "Got control while waiting for shutdown")
                            if ctl.shutdown {
                                break waitloop
                            }
                        }
                    }
                    
                    fmt.Fprintln(os.Stderr, "Re-entering readFifo")
                    // Re-open the file
                    err := readFifo(infile, logReader)
                    if err != nil {
                        // Oops
                        logReader.events <- LogEventMessage{err: fmt.Errorf("Error trying to reopen %s: %s", infile, err.Error())}
                    }
                    
                    close(events)
                    return
                } else if ctl.shutdown {
                    return
                }
            }
        }
    }()
    
    return nil
}

func readFile(infile string, logReader *LogReader) error {
    file, err := os.OpenFile(infile, os.O_RDONLY, 0)
    if err != nil {
        return err
    }
    
    logReader.closed = false
    events := make(chan LogEventMessage)
    
    // Data stream
    go func() {
        // First try to find the end of the file
        stat, err := file.Stat()
        if err != nil {
            events <- LogEventMessage{err: err}
            return
        }
        
        buf := make([]byte, 2048)
        skip := false
        
        // If empty, no need to seek
        if stat.Size() > 0 {
            // Seek near end of file, check for line ending
            file.Seek(1, 2)
            n, err := file.Read(buf[0:1])
            if err != nil {
                file.Close()
                events <- LogEventMessage{err: fmt.Errorf("Error while reading %s: %s", infile, err.Error()), closed: true}
                logReader.control <- LogControlMessage{shutdown: true}
                return
            }
            
            if n == 1 && buf[0] == '\n' {
                skip = false
            } else {
                // Still the end of a partial line, so skip it
                skip = true
            }
        }
        
        var sbuf bytes.Buffer
        
        // Read data
        for {
            n, err := file.Read(buf)
            if err == io.EOF {
                // This is actually how tail -f works, folks
                time.Sleep(time.Duration(200 * time.Millisecond))
                continue
            } else if err != nil {
                file.Close()
                logReader.closed = true
                events <- LogEventMessage{err: fmt.Errorf("Error while reading %s: %s", infile, err.Error()), closed: true}
                logReader.control <- LogControlMessage{shutdown: true}
                return
            }
            
            st := 0
            for st < n {
                idx := bytes.IndexByte(buf[st:n], '\n')
                if idx < 0 {
                    if !skip {
                        sbuf.Write(buf[st:n])
                    }
                    
                    break
                }
                
                if !skip {
                    sbuf.Write(buf[st:st + idx])
                    if sbuf.Len() > 0 {
                        events <- LogEventMessage{data: sbuf.String()}
                        sbuf.Reset()
                    }
                    st += idx + 1
                } else {
                    st += idx + 1
                    skip = false
                }
            }
        }
    }()
    
    // Control stream
    go func() {
        for {
            select {
            case event := <- events:
                // Forward
                logReader.events <- event
            case ctl := <- logReader.control:
                if ctl.close {
                    file.Close()
                }
                
                if ctl.reset {
                    file.Close()
                    
                    // Wait for shutdown
waitloop:           for {
                        select {
                        case event := <- events:
                            if event.data != "" {
                                logReader.events <- event
                            }
                        
                        case ctl := <- logReader.control:
                            if ctl.shutdown {
                                break waitloop
                            }
                        }
                    }
                    
                    // Re-open
                    err := readFile(infile, logReader)
                    if err != nil {
                        logReader.events <- LogEventMessage{err: fmt.Errorf("Error trying to reopn %s: %s", infile, err.Error())}
                    }
                    
                    return
                }
                
                if ctl.shutdown {
                    return
                }
            }
        }
    }()
    
    return nil
}
