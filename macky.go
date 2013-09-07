/*
 * macky - Simple MU* Non-Client
 *
 * See README.md for usage
 *
 * See LICENSE for licensing info
 * 
 * Written Sept 2013 Kutani
 */

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
)

type Server struct {
	path    string
	Address string
	Port    int
	Tls     bool
	Login   string
	User    string
	Pass    string
	netConn net.Conn
	Control chan string
}

func (s *Server) ReadConf(dir string) error {
	f, err := os.Open(fmt.Sprintf("connections/%s/conf", dir))

	if err != nil {
		fmt.Println(err)
		return err
	}
	
	defer f.Close()

	s.path = fmt.Sprint("connections/", dir)

	err = json.NewDecoder(bufio.NewReader(f)).Decode(s)

	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func (s *Server) Connect() error {
	n, err := net.Dial("tcp", fmt.Sprint(s.Address, ":", s.Port))

	if err != nil {
		fmt.Println(err)
		return err
	}

	s.netConn = n

	err = s.build_fifo()

	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func (s *Server) build_fifo() error {
	for {
		err := syscall.Mkfifo(fmt.Sprint(s.path, "/in"), syscall.S_IRWXU)

		if err != nil {
			if os.IsExist(err) {
				err := os.Remove(fmt.Sprint(s.path, "/in"))
				if err != nil {
					return err
				}
				continue
			}
			return err
		}

		break
	}

	return nil
}

func (s *Server) clean_fifo() {
	err := os.Remove(fmt.Sprint(s.path, "/in"))
	if err != nil {
		fmt.Println(err)
	}
}

func (s *Server) Close() {
	s.netConn.Close()
}

func (s *Server) LogIn() {
	cmd := strings.Replace(s.Login, "%u", s.User, -1)
	cmd = strings.Replace(cmd, "%p", s.Pass, -1)

	s.Control <- cmd
}

func (s *Server) ReadIn(out chan<- string) {
	for {
		infile, err := os.OpenFile(fmt.Sprint(s.path, "/in"), os.O_RDONLY, 0666)

		if err != nil {
			fmt.Println(err)
			return
		}

		inbuf := bufio.NewReader(infile)

		msg, err := inbuf.ReadString('\n')

		msg = msg[:len(msg)-1]

		out <- msg

		infile.Close()
	}
}

func (s *Server) WriteOut(in <-chan string) {
	outfile, err := os.OpenFile(fmt.Sprint(s.path, "/out"), os.O_WRONLY|os.O_CREATE, 0666)

	if err != nil {
		fmt.Println(err)
		return
	}

	outbuf := bufio.NewWriter(outfile)

	for {

		out := <-in
		ret, err := outbuf.WriteString(fmt.Sprintln(out))
		outbuf.Flush()

		if ret < len(out) && err != nil {
			fmt.Println(err)
			break
		}

	}
	outfile.Close()
}

func (s *Server) Process(output chan<- string) {
	var rec chan string = make(chan string, 1)
	var snd chan string = make(chan string, 1)
	var in chan string = make(chan string, 1)
	var out chan string = make(chan string, 1)

	go s.Recieve(rec)
	go s.Send(snd)
	go s.ReadIn(in)
	go s.WriteOut(out)

	for {
		select {
		case msg := <-s.Control:
			snd <- msg
		case msg := <-in:
			// Parse command messages here!
			if msg == "CTL_QUIT" {
				output <- msg
				break
			}

			out <- msg
			// output <- msg
			snd <- msg
		case msg := <-rec:
			// Format output here!
			out <- msg
			// output <- msg
		}
	}
}

func (s *Server) Send(in <-chan string) {
	conbuf := bufio.NewWriter(s.netConn)

	for {
		msg := <-in

		ret, err := conbuf.WriteString(fmt.Sprintln(msg))
		conbuf.Flush()

		if ret < len(msg) && err != nil {
			fmt.Println(err)
			break
		}
	}
}

func (s *Server) Recieve(out chan<- string) {
	conbuf := bufio.NewReader(s.netConn)

	for {
		ret, err := conbuf.ReadString('\n')

		if err != nil {
			fmt.Println(err)
			break
		}

		ret = ret[:len(ret)-1]

		out <- ret
	}
}

func main() {
	if _, err := os.Stat("connections"); os.IsNotExist(err) {
		fmt.Println("connections directory does not exist; please see README.md")
		return
	}
	
	if len(os.Args) <= 1 {
		fmt.Println("No connection passed; please see README.md")
		return
	}

	var dir string = os.Args[1]

	serv := new(Server)
	err := serv.ReadConf(dir)

	if err != nil {
		return
	}

	err = serv.Connect()

	if err != nil {
		return
	}

	defer serv.Close()
	defer serv.clean_fifo()

	var output chan string = make(chan string)
	serv.Control = make(chan string)

	go serv.Process(output)

	serv.LogIn()

	for {
		msg := <-output
		fmt.Println(msg)
		if msg == "CTL_QUIT" {
			break
		}
	}

	return
}
