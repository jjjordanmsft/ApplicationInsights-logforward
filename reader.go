
package main

import (
    "bytes"
    "fmt"
    "io"
    "log"
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
    file := os.NewFile(0, "stdin")
    
    stat, err := file.Stat()
    if err != nil {
        return fmt.Errorf("Error stat'ing stdin: %s", err.Error())
    }
    
    if (stat.Mode() & os.ModeCharDevice) != 0 {
        return fmt.Errorf("Refusing to read data from a terminal")
    }
    
    // Data stream
    go func() {
        buf := make([]byte, 2048)
        writer := makeLogEventWriter(logReader.events, 0)
        
        for {
            n, err := file.Read(buf)
            if err == io.EOF || n == 0 {
                break
            }
            
            if err != nil {
                logReader.events <- LogEventMessage{err: fmt.Errorf("Error while reading stdin: %s", err.Error())}
                break
            }
            
            writer.Write(buf[0:n])
        }
        
        log.Print("Exiting read loop")
        
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
                    log.Print("Received close signal")
                    file.Close()
                } else if ctl.shutdown {
                    return
                }
            }
        }
    }()
    
    return nil
}

func readFifo(infile string, logReader *LogReader) error {
    file, err := os.OpenFile(infile, os.O_RDWR, 0)
    if err != nil {
        return err
    }
    
    // FIFOs are tricky.  Here's how they end up working:
    // If opened with O_RDONLY:
    //   - Open blocks waiting for writer.
    //   - File is closed when writer closes, Read is interrupted.
    //   - Read cannot be interrupted by concurrent Close.
    // If opened with O_RDWR (probably Linux-specific behavior):
    //   - Open() does not block.
    //   - File stays open when writer closes, can accept other
    //     writers without reopening.
    //   - Read still cannot be interrupted by concurrent Close.
    // So we pick O_RDWR.  To signal the data goroutine to quit,
    // we need to use a channel to signal interruption + write
    // data to the pipe to get Read() to return.
    
    logReader.closed = false
    events := make(chan LogEventMessage)
    interrupt := make(chan bool, 1)
    
    // Data stream
    go func() {
        buf := make([]byte, 2048)
        writer := makeLogEventWriter(events, 0)
        
wait:	for {
            n, err := file.Read(buf)
            if err == io.EOF || n == 0 {
                break
            }
            
            if err != nil {
                events <- LogEventMessage{err: fmt.Errorf("Error while reading %s: %s", infile, err.Error()), closed: true}
                break
            }
            
            // Check for interrupt signal
            select {
                case <- interrupt:
                    file.Close()
                    break wait
                default:
                    break
            }
            
            writer.Write(buf[0:n])
        }
        
        log.Print("Exited FIFO loop")
        
        logReader.closed = true
        events <- LogEventMessage{closed: true}
        logReader.control <- LogControlMessage{shutdown: true}
    }()
    
    // Control stream
    go func() {
        for {
            select {
            case event := <- events:
                // Forward event
                logReader.events <- event
            case ctl := <- logReader.control:
                if ctl.close {
                    interrupt <- true
                    file.WriteString("\n") // WAKE UP!
                } else if ctl.reset {
                    log.Print("Received reset signal")
                    interrupt <- true
                    file.WriteString("\n") // WAKE UP!
                    
                    // Wait for close
                    for {
                        event := <- events
                        closed := event.closed
                        if event.data != "" {
                            logReader.events <- event
                        }
                        
                        if closed {
                            break
                        }
                    }
                    
                    // Wait for shutdown
                    for {
                        ctl := <- logReader.control
                        if ctl.shutdown {
                            break
                        }
                    }
                    
                    log.Print("Re-entering readFifo")
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
        var skip int
        
        // If empty, no need to seek
        if stat.Size() > 0 {
            // Seek near end of file, check for line ending
            file.Seek(-1, 2)
            n, err := file.Read(buf[0:1])
            if err != nil {
                file.Close()
                events <- LogEventMessage{err: fmt.Errorf("Error while reading %s: %s", infile, err.Error()), closed: true}
                logReader.control <- LogControlMessage{shutdown: true}
                return
            }
            
            if n == 1 && buf[0] == '\n' {
                skip = 0
            } else {
                // Still the end of a partial line, so skip it
                skip = 1
            }
        }
        
        writer := makeLogEventWriter(events, skip)
        
        // Read data
        for {
            n, err := file.Read(buf)
            if err == io.EOF {
                // This is actually how tail -f works, folks
                time.Sleep(time.Duration(200 * time.Millisecond))
                continue
            } else if err != nil {
                log.Printf("Error during read was: %s", err.Error())
                file.Close()
                logReader.closed = true
                events <- LogEventMessage{err: fmt.Errorf("Error while reading %s: %s", infile, err.Error()), closed: true}
                logReader.control <- LogControlMessage{shutdown: true}
                return
            }
            
            writer.Write(buf[0:n])
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
                    
                    // Wait for close
                    for !logReader.closed {
                        event := <- events
                        closed := event.closed
                        if event.data != "" {
                            event.closed = false
                            logReader.events <- event
                        }
                        
                        if closed {
                            break
                        }
                    }
                    
                    // Wait for shutdown
                    for {
                        ctl := <- logReader.control
                        if ctl.shutdown {
                            break
                        }
                    }
                    
                    // Re-open
                    err := readFile(infile, logReader)
                    if err != nil {
                        logReader.events <- LogEventMessage{err: fmt.Errorf("Error trying to reopn %s: %s", infile, err.Error()), closed: true}
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

type logEventWriter struct {
    buffer	bytes.Buffer
    events	chan LogEventMessage
    skip	int
}

func makeLogEventWriter(events chan LogEventMessage, skip int) *logEventWriter {
    return &logEventWriter{events: events, skip: skip}
}

func (writer *logEventWriter) Write(data []byte) {
    for len(data) > 0 {
        idx := bytes.IndexByte(data, '\n')
        if idx < 0 {
            if writer.skip == 0 {
                writer.buffer.Write(data)
            }
            return
        } else {
            if writer.skip == 0 {
                if writer.buffer.Len() == 0 {
                    // Skip writing intermediate to buffer
                    writer.events <- LogEventMessage{data: string(data[0:idx])}
                } else {
                    writer.buffer.Write(data[0:idx])
                    writer.events <- LogEventMessage{data: writer.buffer.String()}
                    writer.buffer.Reset()
                }
            } else {
                writer.skip -= 1
            }
            
            data = data[idx + 1:len(data)]
        }
    }
}
