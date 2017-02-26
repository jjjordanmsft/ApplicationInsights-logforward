# ApplicationInsights-logforward

This is a pair of *experimental* utilities that read log output in real time
from external processes, and forwards that data to Application Insights. 
Input can come from logfiles (which are tailed), stdin, or FIFOs.

For a quick start, the `example` subdirectory contains an example for
building a Docker image that logs all requests.  To build this image:

```sh
	cd ailognginx
	go get && go build -o ../example/ailognginx
	cd ../ailogtrace
	go get && go build -o ../example/ailogtrace
	cd ../example
	docker build -t ainginxexample .
```

then to run it:

```sh
	docker run -it --rm -p 80:80 -e IKEY=<instrumentation key> ainginxexample
```

## ailognginx

This tool processes nginx logs and sends this data as requests.  The usage
is:

```
  -custom value
        Include custom property in telemetry like 'key=value'. Can be used multiple times
  -debug
        Show debugging output
  -endpoint string
        ApplicationInsights ingestion endpoint
  -format string
        nginx log format (required)
  -ikey string
        ApplicationInsights instrumentation key (required)
  -in string
        Input file, or '-' for stdin (required)
  -out string
        Output file, '-' for stdout, 'stderr' for stderr
  -quiet
        Don't write any output messages
  -role string
        Telemetry role name. Defaults to the machine hostname
  -roleinstance string
        Telemetry role instance. Defaults to the machine hostname
```

At a minimum, `-in`, `-format`, and `-ikey` must be specified.  Some options
deserve some elaboration:

* `-format`
Must exactly match the log format specified in the nginx configuration file. 
Naturally, the data that can be sent is limited to what appears in the
logfile.  By default, nginx may not output information such as the virtual
host or scheme.  Therefore, we recommend the following format:

```
$remote_addr - $remote_user [$time_local] $scheme $host "$request" $request_time $status "$http_referer" "$http_x_forwarded_for" "$http_user_agent"
```

A full list of nginx variables can be found [here](http://nginx.org/en/docs/varindex.html)

Many of the common variables will be mapped into Application Insights
telemetry events.  If data is found that cannot be mapped, it will be
included as custom properties.

* `-in`
The input file.  If a regular file is specified, then new events will be read
from the end and already-existing events will be ignored.  If it is a FIFO,
it will read all events sent to it; it will continue to listen if a writer
closes its end.

* `-out`
The output file.  `ailognginx` will write all ingested log data to this file
or FIFO.  This can be thought of being similar to `tee`.

* `-custom`
Add a custom property to all request telemtry.  This argument can be 
specified multiple times.  The value is of the form `key=value`.

* `-role` and `-roleinstance`
Add properties to the telemetry that specify information about the machine
that is running nginx.  As it may be in a container, it's possible to use
these options to inject more meaningful data than the random strings Docker
gives you for the hostname.

## ailogtrace

This tool inputs generic log data and sends them directly to Application
Insights as trace events.  The usage is:

```
  -batch int
        Batch output for n seconds and send as a single trace
  -custom value
        Include custom property in telemetry like 'key=value'. Can be used multiple times
  -debug
        Show debugging output
  -endpoint string
        ApplicationInsights ingestion endpoint
  -exclude value
        Exclude lines that match this regex
  -ikey string
        ApplicationInsights instrumentation key (required)
  -in string
        Input file, or '-' for stdin (required)
  -include value
        Include lines that match this regex
  -out string
        Output file, '-' for stdout, 'stderr' for stderr
  -quiet
        Don't write any output messages
  -role string
        Telemetry role name. Defaults to the machine hostname
  -roleinstance string
        Telemetry role instance. Defaults to the machine hostname
  -severity string
        Severity level in trace telemetry: Verbose, Information, Warning, Error, Critical (default "Information")
```

The only required arguments are `-ikey` and `-in`.

By default, `ailogtrace` will send one trace event per line of input.  If
`-batch N` is specified, then all messages sent within a window of `N`
seconds will be sent together as a single trace event.

Input can be filtered with either or both `-include` and `-exclude` options. 
These specify regular expressions that will either include or skip lines
that match those regular expressions.  They can each be included multiple
times.  Note that these expressions are compared against *lines* rather than
batches.

`-severity` can be used to specify the severity level that appears in the
telemetry.

The other options are the same as above.

## Log rotation

Using regular files as either `-in` or `-out` can be tricky if log rotation
is desired.  Both tools will reopen regular files for both `-in` and `-out`
if signaled with `SIGHUP`.  This should be compatible with standard log
rotation utiltiies, but requires an extra step when configuring them.
