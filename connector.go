package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"golang.org/x/term"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// a abstract redis connection
type Connection struct {
	args      *Args
	conn      net.Conn
	bufReader *bufio.Reader
	connected bool
	istty     bool
	writer    io.Writer
}

func NewConnection(args *Args) *Connection {
	return &Connection{
		args:   args,
		istty:  term.IsTerminal(int(os.Stdout.Fd())),
		writer: os.Stdout,
	}
}

// do connect and auth and select db
func (c *Connection) Connect() error {
	_ = c.Close()
	addr := fmt.Sprintf("%s:%d", c.args.Hostname, c.args.Port)
	var conn net.Conn
	var err error
	if c.args.Tls {
		conf := c.parseTlsConfig()
		conn, err = tls.Dial("tcp", addr, conf)
	} else {
		conn, err = net.Dial("tcp", addr)
	}
	if err != nil {
		fmt.Printf("Could not connect to Redis at %s: %s\n", addr, err.Error())
		return err
	}
	c.connected = true
	c.conn = conn
	c.bufReader = bufio.NewReader(conn)

	err = c.auth()
	if err == nil {
		err = c.selectDb()
	}

	return nil
}

func (c *Connection) parseTlsConfig() *tls.Config {
	config := &tls.Config{
		ServerName:         c.args.Sni,
		InsecureSkipVerify: c.args.Insecure,
	}

	if c.args.Cert != "" {
		caCert, err := tls.LoadX509KeyPair(c.args.Cert, c.args.Key)
		if err != nil {
			log.Fatalf("Failed to load client certificate: %v", err)
		}
		config.Certificates = []tls.Certificate{caCert}
	}

	if c.args.Cacert != "" {
		caCert, err := tls.LoadX509KeyPair(c.args.Cacert, c.args.Key)
		if err != nil {
			log.Fatalf("Failed to load CA certificate: %v", err)
		}
		config.RootCAs.AppendCertsFromPEM(caCert.Certificate[0])
	}

	if c.args.Cacertdir != "" {
		config.ClientCAs = loadCACertificates(c.args.Cacertdir)
	}

	if c.args.TlsCiphers != "" {
		config.CipherSuites = parseCiphers(c.args.TlsCiphers)
	}

	if c.args.TlsCiphersuites != "" {
		config.CipherSuites = parseCipherSuites(c.args.TlsCiphersuites)
	}
	return config
}

func loadCACertificates(dir string) *x509.CertPool {
	pool := x509.NewCertPool()
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("Failed to read directory: %v", err)
	}

	for _, file := range files {
		cert, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			log.Printf("Failed to read certificate file %s: %v", file.Name(), err)
			continue
		}
		if !pool.AppendCertsFromPEM(cert) {
			log.Printf("Failed to add certificate from file %s", file.Name())
		}
	}

	return pool
}

func parseCiphers(ciphers string) []uint16 {
	// Implement parsing logic for TLS ciphers
	return nil
}

func parseCipherSuites(ciphersuites string) []uint16 {
	// Implement parsing logic for TLS ciphersuites
	return nil
}

func (c *Connection) Exec(input string) (*TypedVal, error) {
	err := c.Send(input)
	if err != nil {
		c.PrintRawString(err.Error())
		return nil, err
	}
	tv, err := c.ReceiveValue()
	if err != nil {
		c.PrintRawString(err.Error())
		return nil, err
	}
	if tv.Type == TypeError {
		strings.Fields(tv.Val.(string))
		// todo
	}
	return tv, nil
}

func (c *Connection) auth() error {
	var pass string
	if c.args.Askpass {
		fmt.Print("Please input password: ")
		// TODO: this is different with redis-cli, we can't echo *
		passBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}
		pass = string(passBytes)
		fmt.Println()
	} else {
		// consider password from args or env
		pass = defaults(c.args.Pass, c.args.Password, os.Getenv("REDISCLI_AUTH"))
	}
	if pass == "" {
		return nil
	}
	tv, err := c.Exec(fmt.Sprintf("AUTH %s %s", c.args.User, pass))
	if err != nil {
		c.PrintRawString(err.Error())
		return err
	}
	if tv.Type == TypeError {
		_, _ = fmt.Fprintf(c.writer, "AUTH failed: %s\n", tv.Val)
	}
	return nil
}

func (c *Connection) selectDb() error {
	if c.args.Db == 0 {
		return nil
	}
	tv, err := c.Exec("SELECT " + fmt.Sprint(c.args.Db))
	if err != nil {
		c.PrintRawString(err.Error())
		return err
	}
	if tv.Type == TypeError {
		_, _ = fmt.Fprintf(c.writer, "SELECT %d failed: %s\n", c.args.Db, tv.Val)
		c.args.Db = 0
	}
	return nil
}

func (c *Connection) Close() error {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.bufReader = nil
	c.connected = false
	return nil
}

// always send command with \r\n (some redis server may not support \n)
func (c *Connection) Send(input string) (err error) {
	_, err = c.conn.Write([]byte(input + "\r\n"))
	return
}

func (c *Connection) ReceiveValue() (*TypedVal, error) {
	return ReadValue(c.bufReader)
}

// print value with format or not , by args --no-raw
// and, if not tty, always print in raw format
func (c *Connection) PrintVal(tv *TypedVal) {
	if c.args.NoRaw {
		PrintVal(c.writer, tv, false)
	} else {
		PrintVal(c.writer, tv, c.args.Raw || !c.istty)
	}
}

func (c *Connection) PrintRawString(str string) {
	_, _ = fmt.Fprint(c.writer, str)
}

func (c *Connection) CliPrefix() string {
	if !c.connected {
		return "not connected"
	}
	addr := fmt.Sprintf("%s:%d", c.args.Hostname, c.args.Port)
	if c.args.Db != 0 {
		return fmt.Sprintf("%s[%d]", addr, c.args.Db)
	} else {
		return addr
	}
}

// exec command and print result with format
func (c *Connection) ExecPrint(input string) error {
	tv, err := c.Exec(input)
	if err != nil {
		return err
	}
	if isCmd(input, "info") {
		// always print info command raw string
		c.PrintRawString(tv.Val.(string))
	} else {
		c.PrintVal(tv)
	}
	if isCmd(input, "select") && tv.Val.(string) == "OK" {
		// update completer prefix
		c.args.Db, _ = strconv.Atoi(strings.Fields(input)[1])
	}
	return nil
}

func defaults(str ...string) string {
	for _, s := range str {
		if s != "" {
			return s
		}
	}
	return ""
}
