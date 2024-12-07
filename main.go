package main

import (
	"context"
	"fmt"
	"github.com/c-bata/go-prompt"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Args struct {
	Hostname           string  `flag:"h" default:"127.0.0.1" desc:"Server hostname"`
	Port               int     `flag:"p" default:"6379" desc:"Server port"`
	Socket             string  `flag:"s" desc:"Server socket (overrides hostname and port)"`
	Password           string  `flag:"a" desc:"Password to use when connecting to the server"`
	User               string  `flag:"user" desc:"Used to send ACL style 'AUTH username pass'. Needs -a."`
	Pass               string  `flag:"pass" desc:"Alias of -a for consistency with the new --user option"`
	Askpass            bool    `flag:"askpass" desc:"Force user to input password with mask from STDIN"`
	Uri                string  `flag:"u" desc:"Server URI"`
	Repeat             int     `flag:"r" default:"1" desc:"Execute specified command N times"`
	Interval           float64 `flag:"i" default:"0" desc:"Interval between commands when using -r"`
	Db                 int     `flag:"n" default:"0" desc:"Database number"`
	Resp2              bool    `flag:"2" desc:"Start session in RESP2 protocol mode"`
	Resp3              bool    `flag:"3" desc:"Start session in RESP3 protocol mode"`
	ReadLastArg        bool    `flag:"x" desc:"Read last argument from STDIN"`
	ReadTagArg         string  `flag:"X" desc:"Read <tag> argument from STDIN"`
	DelimiterBulk      string  `flag:"d" default:"\n" desc:"Delimiter between response bulks for raw formatting"`
	DelimiterResponses string  `flag:"D" default:"\n" desc:"Delimiter between responses for raw formatting"`
	ClusterMode        bool    `flag:"c" desc:"Enable cluster mode"`
	ExitError          bool    `flag:"e" desc:"Return exit error code when command execution fails"`
	Tls                bool    `flag:"tls" desc:"Establish a secure TLS connection"`
	Sni                string  `flag:"sni" desc:"Server name indication for TLS"`
	Cacert             string  `flag:"cacert" desc:"CA Certificate file to verify with"`
	Cacertdir          string  `flag:"cacertdir" desc:"Directory where trusted CA certificates are stored"`
	Insecure           bool    `flag:"insecure" desc:"Allow insecure TLS connection by skipping cert validation"`
	Cert               string  `flag:"cert" desc:"Client certificate to authenticate with"`
	Key                string  `flag:"key" desc:"Private key file to authenticate with"`
	TlsCiphers         string  `flag:"tls-ciphers" desc:"Sets the list of preferred ciphers (TLSv1.2 and below)"`
	TlsCiphersuites    string  `flag:"tls-ciphersuites" desc:"Sets the list of preferred ciphersuites (TLSv1.3)"`
	Raw                bool    `flag:"raw" desc:"Use raw formatting for replies"`
	NoRaw              bool    `flag:"no-raw" desc:"Force formatted output"`
	QuotedInput        bool    `flag:"quoted-input" desc:"Force input to be handled as quoted strings"`
	Csv                bool    `flag:"csv" desc:"Output in CSV format"`
	Json               bool    `flag:"json" desc:"Output in JSON format"`
	QuotedJson         bool    `flag:"quoted-json" desc:"Produce ASCII-safe quoted strings, not Unicode"`
	ShowPushes         string  `flag:"show-pushes" default:"yes" desc:"Whether to print RESP3 PUSH messages"`
	Stat               bool    `flag:"stat" desc:"Print rolling stats about server"`
	Latency            bool    `flag:"latency" desc:"Enter a special mode continuously sampling latency"`
	LatencyHistory     bool    `flag:"latency-history" desc:"Like --latency but tracking latency changes over time"`
	LatencyDist        bool    `flag:"latency-dist" desc:"Shows latency as a spectrum"`
	LruTest            int     `flag:"lru-test" default:"0" desc:"Simulate a cache workload with an 80-20 distribution"`
	Replica            bool    `flag:"replica" desc:"Simulate a replica showing commands received from the master"`
	Rdb                string  `flag:"rdb" desc:"Transfer an RDB dump from remote server to local file"`
	FunctionsRdb       string  `flag:"functions-rdb" desc:"Like --rdb but only get the functions"`
	Pipe               bool    `flag:"pipe" desc:"Transfer raw Redis protocol from stdin to server"`
	PipeTimeout        int     `flag:"pipe-timeout" default:"30" desc:"In --pipe mode, abort with error if no reply is received"`
	Bigkeys            bool    `flag:"bigkeys" desc:"Sample Redis keys looking for keys with many elements"`
	Memkeys            bool    `flag:"memkeys" desc:"Sample Redis keys looking for keys consuming a lot of memory"`
	MemkeysSamples     int     `flag:"memkeys-samples" default:"0" desc:"Number of key elements to sample"`
	Hotkeys            bool    `flag:"hotkeys" desc:"Sample Redis keys looking for hot keys"`
	Scan               bool    `flag:"scan" desc:"List all keys using the SCAN command"`
	Pattern            string  `flag:"pattern" default:"*" desc:"Keys pattern when using the --scan, --bigkeys or --hotkeys"`
	Count              int     `flag:"count" default:"10" desc:"Count option when using the --scan, --bigkeys or --hotkeys"`
	QuotedPattern      string  `flag:"quoted-pattern" desc:"Same as --pattern, but the specified string can be quoted"`
	IntrinsicLatency   int     `flag:"intrinsic-latency" default:"0" desc:"Run a test to measure intrinsic system latency"`
	Eval               string  `flag:"eval" desc:"Send an EVAL command using the Lua script at <file>"`
	Ldb                bool    `flag:"ldb" desc:"Used with --eval enable the Redis Lua debugger"`
	LdbSyncMode        bool    `flag:"ldb-sync-mode" desc:"Like --ldb but uses the synchronous Lua debugger"`
	Cluster            string  `flag:"cluster" desc:"Cluster Manager command and arguments"`
	Verbose            bool    `flag:"verbose" desc:"Verbose mode"`
	NoAuthWarning      bool    `flag:"no-auth-warning" desc:"Don't show warning when using password"`
	Help               bool    `flag:"help" desc:"Output this help and exit"`
	Version            bool    `flag:"version" desc:"Output version and exit"`
}

var args = &Args{}

var connection *Connection

func main() {
	restArgs := parseArgs(args)
	//debugPrintArgs(args)
	if args.Help {
		printHelp()
		return
	}
	var err error
	if args.Scan {
		err = scan()
	} else if len(restArgs) > 0 {
		// redis-cli -h xx -p xx -a xx cmd arg1 arg2 ...
		// restArgs = [cmd arg1 arg2 ...]
		// since first arg that not defined, will be use as command and it's args
		err = singleCmd(func(connection *Connection) error {
			return connection.ExecPrint(strings.Join(restArgs, " "))
		})
	} else {
		interactive()
	}
	if err != nil && args.ExitError {
		os.Exit(1)
	}
}

// main loop for interactive mode
func interactive() {
	ctx, stop := signal.NotifyContext(context.TODO(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	connection = NewConnection(args)
	defer connection.Close()

	go func() {
		connection.Connect()
		p := prompt.New(executor, completer, prompt.OptionPrefix(connection.CliPrefix()+"> "), prompt.OptionLivePrefix(func() (string, bool) {
			return connection.CliPrefix() + "> ", true
		}))
		p.Run()
	}()

	<-ctx.Done()
	stop()
}

func executor(input string) {
	if !connection.connected {
		err := connection.Connect()
		if err != nil {
			return
		}
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}
	if isCmd(input, "exit") || isCmd(input, "quit") {
		os.Exit(0)
	}
	if err := connection.ExecPrint(input); err != nil {
		fmt.Println(err.Error())
	}
}

// check if input is specific command or not
func isCmd(input, cmd string) bool {
	return strings.EqualFold(strings.Fields(input)[0], cmd)
}

// todo: add more commands
func completer(d prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{
		//{Text: "select", Description: "Store the username and age"},
	}
	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
}

// struct field and it's reflect value
type fv struct {
	f   reflect.StructField
	v   reflect.Value
	set bool
}

func (fv0 *fv) SetValue(str string) {
	switch fv0.f.Type.Kind() {
	case reflect.String:
		fv0.v.SetString(str)
	case reflect.Int:
		v, _ := strconv.Atoi(str)
		fv0.v.SetInt(int64(v))
	case reflect.Float64:
		v, _ := strconv.ParseFloat(str, 64)
		fv0.v.SetFloat(v)
	case reflect.Bool:
		fv0.v.SetBool(true)
	default:
		fmt.Println("unsupported args type:", fv0.f.Type.Kind())
	}
	fv0.set = true
}

// here we parse args from os.Args
func parseArgs(ptr any) (rest []string) {

	fieldsMap := map[string]*fv{}
	forEachExportedField(ptr, func(f reflect.StructField, v reflect.Value) {
		fieldsMap[f.Tag.Get("flag")] = &fv{f: f, v: v, set: false}
	})
	var i = 1
	for ; i < len(os.Args); i++ {
		a := os.Args[i]
		if strings.HasPrefix(a, "-") {
			key := strings.TrimLeft(a, "-")
			if fv, ok := fieldsMap[key]; ok {
				if fv.f.Type.Kind() == reflect.Bool {
					fv.SetValue("")
				} else if i+1 < len(os.Args) {
					fv.SetValue(os.Args[i+1])
					i++
					continue
				} else {
					printHelp()
					os.Exit(0)
				}
			} else {
				fmt.Printf("Unrecognized option or bad number of args for: '%s'\n", a)
				os.Exit(0)
			}
		} else {
			break
		}
	}
	for _, fv := range fieldsMap {
		if !fv.set {
			if defVal := fv.f.Tag.Get("default"); defVal != "" {
				fv.SetValue(defVal)
			}
		}
	}
	if i >= len(os.Args) {
		return
	}
	return os.Args[i:]
}

func printHelp() {
	// msg is copy from redis-cli help
	fmt.Println(`
Usage: redis-cli [OPTIONS] [cmd [arg [arg ...]]]
  -h <hostname>      Server hostname (default: 127.0.0.1).
  -p <port>          Server port (default: 6379).
  -s <socket>        Server socket (overrides hostname and port).
  -a <password>      Password to use when connecting to the server.
                     You can also use the REDISCLI_AUTH environment
                     variable to pass this password more safely
                     (if both are used, this argument takes precedence).
  --user <username>  Used to send ACL style 'AUTH username pass'. Needs -a.
  --pass <password>  Alias of -a for consistency with the new --user option.
  --askpass          Force user to input password with mask from STDIN.
                     If this argument is used, '-a' and REDISCLI_AUTH
                     environment variable will be ignored.
  -u <uri>           Server URI.
  -r <repeat>        Execute specified command N times.
  -i <interval>      When -r is used, waits <interval> seconds per command.
                     It is possible to specify sub-second times like -i 0.1.
                     This interval is also used in --scan and --stat per cycle.
                     and in --bigkeys, --memkeys, and --hotkeys per 100 cycles.
  -n <db>            Database number.
  -2                 Start session in RESP2 protocol mode.
  -3                 Start session in RESP3 protocol mode.
  -x                 Read last argument from STDIN (see example below).
  -X                 Read <tag> argument from STDIN (see example below).
  -d <delimiter>     Delimiter between response bulks for raw formatting (default: \n).
  -D <delimiter>     Delimiter between responses for raw formatting (default: \n).
  -c                 Enable cluster mode (follow -ASK and -MOVED redirections).
  -e                 Return exit error code when command execution fails.
  --tls              Establish a secure TLS connection.
  --sni <host>       Server name indication for TLS.
  --cacert <file>    CA Certificate file to verify with.
  --cacertdir <dir>  Directory where trusted CA certificates are stored.
                     If neither cacert nor cacertdir are specified, the default
                     system-wide trusted root certs configuration will apply.
  --insecure         Allow insecure TLS connection by skipping cert validation.
  --cert <file>      Client certificate to authenticate with.
  --key <file>       Private key file to authenticate with.
  --tls-ciphers <list> Sets the list of preferred ciphers (TLSv1.2 and below)
                     in order of preference from highest to lowest separated by colon (":").
                     See the ciphers(1ssl) manpage for more information about the syntax of this string.
  --tls-ciphersuites <list> Sets the list of preferred ciphersuites (TLSv1.3)
                     in order of preference from highest to lowest separated by colon (":").
                     See the ciphers(1ssl) manpage for more information about the syntax of this string,
                     and specifically for TLSv1.3 ciphersuites.
  --raw              Use raw formatting for replies (default when STDOUT is
                     not a tty).
  --no-raw           Force formatted output even when STDOUT is not a tty.
  --quoted-input     Force input to be handled as quoted strings.
  --csv              Output in CSV format.
  --json             Output in JSON format (default RESP3, use -2 if you want to use with RESP2).
  --quoted-json      Same as --json, but produce ASCII-safe quoted strings, not Unicode.
  --show-pushes <yn> Whether to print RESP3 PUSH messages.  Enabled by default when
                     STDOUT is a tty but can be overridden with --show-pushes no.
  --stat             Print rolling stats about server: mem, clients, ...
  --latency          Enter a special mode continuously sampling latency.
                     If you use this mode in an interactive session it runs
                     forever displaying real-time stats. Otherwise if --raw or
                     --csv is specified, or if you redirect the output to a non
                     TTY, it samples the latency for 1 second (you can use
                     -i to change the interval), then produces a single output
                     and exits.
  --latency-history  Like --latency but tracking latency changes over time.
                     Default time interval is 15 sec. Change it using -i.
  --latency-dist     Shows latency as a spectrum, requires xterm 256 colors.
                     Default time interval is 1 sec. Change it using -i.
  --lru-test <keys>  Simulate a cache workload with an 80-20 distribution.
  --replica          Simulate a replica showing commands received from the master.
  --rdb <filename>   Transfer an RDB dump from remote server to local file.
                     Use filename of "-" to write to stdout.
  --functions-rdb <filename> Like --rdb but only get the functions (not the keys)
                     when getting the RDB dump file.
  --pipe             Transfer raw Redis protocol from stdin to server.
  --pipe-timeout <n> In --pipe mode, abort with error if after sending all data.
                     no reply is received within <n> seconds.
                     Default timeout: 30. Use 0 to wait forever.
  --bigkeys          Sample Redis keys looking for keys with many elements (complexity).
  --memkeys          Sample Redis keys looking for keys consuming a lot of memory.
  --memkeys-samples <n> Sample Redis keys looking for keys consuming a lot of memory.
                     And define number of key elements to sample
  --hotkeys          Sample Redis keys looking for hot keys.
                     only works when maxmemory-policy is *lfu.
  --scan             List all keys using the SCAN command.
  --pattern <pat>    Keys pattern when using the --scan, --bigkeys or --hotkeys
                     options (default: *).
  --count <count>    Count option when using the --scan, --bigkeys or --hotkeys (default: 10).
  --quoted-pattern <pat> Same as --pattern, but the specified string can be
                         quoted, in order to pass an otherwise non binary-safe string.
  --intrinsic-latency <sec> Run a test to measure intrinsic system latency.
                     The test will run for the specified amount of seconds.
  --eval <file>      Send an EVAL command using the Lua script at <file>.
  --ldb              Used with --eval enable the Redis Lua debugger.
  --ldb-sync-mode    Like --ldb but uses the synchronous Lua debugger, in
                     this mode the server is blocked and script changes are
                     not rolled back from the server memory.
  --cluster <command> [args...] [opts...]
                     Cluster Manager command and arguments (see below).
  --verbose          Verbose mode.
  --no-auth-warning  Don't show warning message when using password on command
                     line interface.
  --help             Output this help and exit.
  --version          Output version and exit.

Cluster Manager Commands:
  Use --cluster help to list all available cluster manager commands.

Examples:
  cat /etc/passwd | redis-cli -x set mypasswd
  redis-cli -D "" --raw dump key > key.dump && redis-cli -X dump_tag restore key2 0 dump_tag replace < key.dump
  redis-cli -r 100 lpush mylist x
  redis-cli -r 100 -i 1 info | grep used_memory_human:
  redis-cli --quoted-input set '"null-\x00-separated"' value
  redis-cli --eval myscript.lua key1 key2 , arg1 arg2 arg3
  redis-cli --scan --pattern '*:12345*'
  redis-cli --scan --pattern '*:12345*' --count 100

  (Note: when using --eval the comma separates KEYS[] from ARGV[] items)

When no command is given, redis-cli starts in interactive mode.
Type "help" in interactive mode for information on available commands
and settings.`)
}

func debugPrintArgs(ptr any) {
	argsToPrint := [][]string{}
	maxWidth := 0
	forEachExportedField(ptr, func(f reflect.StructField, v reflect.Value) {
		argName := f.Tag.Get("flag")
		desc := f.Tag.Get("desc")
		kv := fmt.Sprintf("-%s=%v", argName, v.Interface())
		kv = strings.ReplaceAll(kv, "\n", `\n`)
		kv = strings.ReplaceAll(kv, "\r", `\r`)
		kv = strings.ReplaceAll(kv, "\t", `\t`)
		if len(kv) > maxWidth {
			maxWidth = len(kv)
		}
		argsToPrint = append(argsToPrint, []string{kv, desc})
	})
	for _, arg := range argsToPrint {
		fmt.Printf("%-*s %s\n", maxWidth, arg[0], arg[1])
	}
}

func forEachExportedField(ptr any, visitor func(f reflect.StructField, v reflect.Value)) {
	rv := reflect.Indirect(reflect.ValueOf(ptr))
	tp := rv.Type()
	for i := 0; i < tp.NumField(); i++ {
		f := tp.Field(i)
		if f.IsExported() {
			v := rv.Field(i)
			visitor(f, v)
		}
	}
}

func singleCmd(exeFunc func(connection *Connection) error) error {
	connection = NewConnection(args)
	defer connection.Close()
	if err := connection.Connect(); err != nil {
		return err
	} else {
		// repeat command with interval
		if args.Repeat == 0 {
			return exeFunc(connection)
		}
		dua := time.Nanosecond * time.Duration(args.Interval*float64(time.Second))
		for i := 0; i < args.Repeat; i++ {
			err = exeFunc(connection)
			if err != nil {
				return err
			}
			if i < args.Repeat-1 && dua > 0 {
				time.Sleep(dua)
			}
		}
		return nil
	}
}

func scan() error {
	return singleCmd(func(connection *Connection) error {
		cursor := "0"
		for {
			tv, err := connection.Exec(fmt.Sprintf("SCAN %s MATCH %s COUNT %d", cursor, args.Pattern, args.Count))
			if err != nil {
				return err
			}
			for _, item := range tv.Val.([]*TypedVal)[1].Val.([]*TypedVal) {
				connection.PrintVal(item)
			}
			cursor = tv.Val.([]*TypedVal)[0].Val.(string)
			if cursor == "0" {
				break
			}
		}
		return nil
	})
}
