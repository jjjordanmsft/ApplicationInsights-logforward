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
        ApplicationInsights ingestion endpoint (optional)
  -format string
        nginx log format
  -ikey string
        ApplicationInsights instrumentation key
  -in string
        Input file, or '-' for stdin
  -jsonescape
        whether the nginx log is JSON-escaped
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

Many of the common variables will be mapped into Application Insights data. 
If data is included that cannot be mapped into that schema, it will be
included as a custom property.

* `-in`
The input file.  This can be a standard file, in which case it will be
continually read from the end; or it can be a FIFO, in which case it will be
read continuously.

* `-out`
The output file.  `ailognginx` will write all ingested log data to this file
or FIFO.  This can be thought of being similar to `tee`.

* `-custom`
Add a custom property to all request telemtry.  This argument can be 
specified multiple times, and the value is of the form `key=value`.

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
        ApplicationInsights ingestion endpoint (optional)
  -filter value
        Include lines that match this regex
  -filterout value
        Discard lines that match this regex
  -ikey string
        ApplicationInsights instrumentation key
  -in string
        Input file, or '-' for stdin
  -out string
        Output file, '-' for stdout, 'stderr' for stderr
  -quiet
        Don't write any output messages
  -role string
        Telemetry role name. Defaults to the machine hostname
  -roleinstance string
        Telemetry role instance. Defaults to the machine hostname
  -severity string
        Severity level in trace telemetry: Verbose, Information, Warning, Error, Critical (default "information")
```

At least `-ikey` and `-in` are required arguments.

By default, `ailogtrace` will send one trace event per line from the input. 
If `-batch N` is specified, then all messages sent within a window
of `N` seconds will be sent together as a single trace event.

Input can be filtered with either or both `-filter` and `-filterout`
options.  These specify regular expressions that will either include or skip
lines that match those regular expressions.

`-severity` can be used to specify the severity level that appears in the
telemetry.

The other options are the same as above.

## Log rotation

Using regular files as either `-in` or `-out` can be tricky if log rotation
is desired.  Both tools will reopen regular files for both `-in` and `-out`
if signaled with `SIGHUP`.  This should be compatible with standard log
rotation utiltiies, but requires an extra step when configuring them.
