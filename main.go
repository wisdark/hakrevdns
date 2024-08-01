package main

import (
	"bufio"
	"context"
	"fmt"
	flags "github.com/jessevdk/go-flags"
	"net"
	"os"
	"strings"
	"sync"
)

var opts struct {
	Threads      int    `short:"t" long:"threads" default:"8" description:"How many threads should be used"`
	ResolverIP   string `short:"r" long:"resolver" description:"IP of the DNS resolver to use for lookups"`
	ResolverFile string `short:"R" long:"resolvers-file" description:"File containing list of DNS resolvers to use for lookups"`
	UseDefault   bool   `short:"U" long:"use-default" description:"Use default resolvers for lookups"`
	Protocol     string `short:"P" long:"protocol" choice:"tcp" choice:"udp" default:"udp" description:"Protocol to use for lookups"`
	Port         uint16 `short:"p" long:"port" default:"53" description:"Port to bother the specified DNS resolver on"`
	Domain       bool   `short:"d" long:"domain" description:"Output only domains"`
	Help         bool   `short:"h" long:"help" description:"Show help message"`
}

var defaultResolvers = []string{
	"1.1.1.1", "1.0.0.1", "8.8.8.8", "8.8.4.4", "9.9.9.9", "149.112.112.112",
	"208.67.222.222", "208.67.220.220", "64.6.64.6", "64.6.65.6", "198.101.242.72",
	"198.101.242.72", "8.26.56.26", "8.20.247.20", "185.228.168.9", "185.228.169.9",
	"76.76.19.19", "76.223.122.150", "94.140.14.14", "94.140.15.15",
}

func main() {
	parser := flags.NewParser(&opts, flags.Default)
	_, err := parser.Parse()

	if err != nil {
		os.Exit(1)
	}

	if opts.Help {
		parser.WriteHelp(os.Stdout)
		os.Exit(0)
	}

	var resolvers []string
	if opts.ResolverFile != "" {
		file, err := os.Open(opts.ResolverFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open resolvers file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			resolvers = append(resolvers, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read resolvers file: %v\n", err)
			os.Exit(1)
		}
	}

	if opts.ResolverIP != "" {
		resolvers = append(resolvers, opts.ResolverIP)
	}

	if opts.UseDefault {
		resolvers = append(resolvers, defaultResolvers...)
	}

	numWorkers := opts.Threads

	work := make(chan string)
	go func() {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			work <- s.Text()
		}
		close(work)
	}()

	wg := &sync.WaitGroup{}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go doWork(work, wg, resolvers)
	}
	wg.Wait()
}

func doWork(work chan string, wg *sync.WaitGroup, resolvers []string) {
	defer wg.Done()

	for ip := range work {
		resolved := false

		for _, resolverIP := range resolvers {
			r := &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					d := net.Dialer{}
					return d.DialContext(ctx, opts.Protocol, fmt.Sprintf("%s:%d", resolverIP, opts.Port))
				},
			}

			addr, err := r.LookupAddr(context.Background(), ip)
			if err == nil {
				for _, a := range addr {
					if opts.Domain {
						fmt.Println(strings.TrimRight(a, "."))
					} else {
						fmt.Println(ip, "\t", a)
					}
				}
				resolved = true
				break
			}
		}

		if !resolved {
			// Uncomment this line if you want to see unresolved IPs
			// fmt.Fprintf(os.Stderr, "Failed to resolve IP: %s\n", ip)
		}
	}
}
