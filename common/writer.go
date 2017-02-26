
package common

import (
    "bytes"
    "fmt"
    "io"
    "log"
    "os"
    "time"
)

const (
    BUF_MAX = 8192
    CHAN_BUF = 1
)

type LogWriter struct {
    events	chan LogWriterEvent
    control	chan LogWriterControl
}

type LogWriterEvent struct {
    closed	bool
    err		error
    ready	bool
}

type LogWriterControl struct {
    data	string
    reset	bool
    shutdown	bool
}

type logOpener func() (io.WriteCloser, error)

type logWriterInternal struct {
    events	chan LogWriterEvent
    control	chan LogWriterControl
    opener	logOpener
    canReset	bool
    file	io.WriteCloser
}

func (logWriter *LogWriter) Reset() {
    logWriter.control <- LogWriterControl{reset: true}
}

func (logWriter *LogWriter) Close() {
    logWriter.control <- LogWriterControl{shutdown: true}
}

func (logWriter *LogWriter) Write(msg string) {
    logWriter.control <- LogWriterControl{data: msg}
}

func NewLogWriter(outfile string) (*LogWriter, error) {
    logWriter := &LogWriter{make(chan LogWriterEvent), make(chan LogWriterControl)}
    
    if outfile == "-" {
        startLogWriter(logWriter, func () (io.WriteCloser, error) { return os.Stdout, nil }, false)
    } else if outfile == "stderr" {
        startLogWriter(logWriter, func () (io.WriteCloser, error) { return os.Stderr, nil }, false)
    } else {
        stat, err := os.Stat(outfile)
        if err != nil {
            // Couldn't stat, so let's try a test-run to create it.
            
            tstf, err := openLogOut(outfile)
            if err != nil {
                return nil, err
            }
            
            tstf.Close()
        } else {
            // If it isn't a named pipe, then let's try to open it (named pipes may hang, or confuse reader).
            if (stat.Mode() & os.ModeNamedPipe) == 0 {
                tstf, err := openLogOut(outfile)
                if err != nil {
                    return nil, err
                }
                
                tstf.Close()
            }
        }
        
        startLogWriter(logWriter, func() (io.WriteCloser, error) { return openLogOut(outfile) }, true)
    }
    
    return logWriter, nil
}

func NewNilLogWriter() *LogWriter {
    logWriter := &LogWriter{make(chan LogWriterEvent), make(chan LogWriterControl)}
    
    go func() {
        for {
            ctl := <- logWriter.control
            if ctl.shutdown {
                logWriter.events <- LogWriterEvent{closed: true}
                return
            }
        }
    }()
    
    return logWriter
}

func openLogOut(outfile string) (io.WriteCloser, error) {
    file, err := os.OpenFile(outfile, os.O_WRONLY | os.O_CREATE | os.O_EXCL, os.ModeAppend | 0666)
    if err != nil {
        log.Printf("Failed to create %s, will try to open normally: %s", outfile, err.Error())
        // Can't create the file, so try to just append.
        file, err = os.OpenFile(outfile, os.O_WRONLY | os.O_APPEND, os.ModeAppend | 0666)
        if err != nil {
            return nil, err
        }
    }
    
    return file, nil
}

func startLogWriter(logWriter *LogWriter, opener logOpener, canReset bool) {
    internal := &logWriterInternal{
        events: make(chan LogWriterEvent, CHAN_BUF),
        control: make(chan LogWriterControl),
        opener: opener,
        canReset: canReset,
        file: nil,
    }
    
    go internal.writerThread()
    go internal.controlThread(logWriter)
}

func (internal *logWriterInternal) writerThread() {
    file, err := internal.opener()
    if err != nil {
        internal.events <- LogWriterEvent{err: err, closed: true}
        return
    }
    
    internal.file = file
    
    for i := 0; i < CHAN_BUF; i++ {
        internal.events <- LogWriterEvent{ready: true}
    }
    
    for {
        internal.events <- LogWriterEvent{ready: true}
        ctl := <- internal.control
        
        if ctl.data != "" {
            st := 0
            for st < len(ctl.data) {
                n, err := io.WriteString(file, ctl.data[st:len(ctl.data)])
                if err != nil {
                    file.Close()
                    internal.events <- LogWriterEvent{err: err, closed: true}
                    return
                }
                
                if n <= 0 {
                    file.Close()
                    internal.events <- LogWriterEvent{err: fmt.Errorf("0 bytes written"), closed: true}
                    return
                }
                
                st += n
            }
        }
        
        if ctl.shutdown {
            file.Close()
            internal.events <- LogWriterEvent{closed: true}
            return
        }
    }
}

func (internal *logWriterInternal) controlThread(logWriter *LogWriter) {
    var buf bytes.Buffer
    var ready int
    var droppedBytes, droppedMsgs int
    
    // Notify about dropped data every minute
    notifier := time.Tick(time.Minute)
    
    for {
        select {
        case evt := <- internal.events:
            if evt.ready {
                if buf.Len() > 0 {
                    internal.control <- LogWriterControl{data: buf.String()}
                    buf.Reset()
                } else {
                    ready += 1
                }
            } else {
                logWriter.events <- evt
                return
            }
        case ctl := <- logWriter.control:
            if ctl.data != "" {
                if ready > 0 {
                    // Writer is ready
                    internal.control <- ctl
                    ready -= 1
                } else if buf.Len() < BUF_MAX {
                    // Not accepting data right now, buffer it.
                    buf.WriteString(ctl.data)
                } else {
                    // We have to drop data at this point.
                    droppedBytes += len(ctl.data)
                    droppedMsgs += 1
                }
            }
            
            if ctl.shutdown {
                if internal.file != nil {
                    internal.file.Close()
                }
                internal.control <- ctl
            }
            
            if ctl.reset {
                if internal.canReset {
                    // Send shutdown control, let it flush out, for up to a second.
                    timeout := time.After(time.Second)
                    
                    if internal.file != nil {
                        internal.file.Close()
                    }
                    
                    internal.control <- LogWriterControl{shutdown: true}
wait:               for {
                        select {
                        case evt := <- internal.events:
                            if evt.closed {
                                break wait
                            }
                        case _ = <- timeout:
                            // Give up.
                            break wait
                        }
                    }
                    
                    startLogWriter(logWriter, internal.opener, internal.canReset)
                    close(internal.events)
                    close(internal.control)
                    return
                }
            }
        case _ = <- notifier:
            if droppedMsgs > 0 {
                logWriter.events <- LogWriterEvent{
                    err: fmt.Errorf("Dropped %d messages and %d bytes in the last minute", droppedMsgs, droppedBytes)}
                droppedMsgs = 0
                droppedBytes = 0
            }
        }
    }
}
