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
	"container/list"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
)

var sList = list.New()
var sListAdd chan *Server = make(chan *Server, 1)
var sListDel chan *Server = make(chan *Server, 1)

func ListHandler(add <-chan *Server, del <-chan *Server) {
	for {
		select {
		case svr := <-add:
			svr.e = sList.PushFront(svr)
		case svr := <-del:
			sList.Remove(svr.e)
		}
	}
}

func SessionExists(s string) bool {
	for e := sList.Front(); e != nil; e = e.Next() {
		if e.Value.(*Server).path == fmt.Sprint("connections/", s) {
			return true
		}
	}
	return false
}

func CloseAllSessions() {
	fmt.Println("Closing all sessions")
	for e := sList.Front(); e != nil; e = e.Next() {
		e.Value.(*Server).Close()
	}
}

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
	e       *list.Element
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

	sListAdd <- s

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
	fmt.Println(s.path, ".Close()")
	s.netConn.Close()
	s.clean_fifo()
	sListDel <- s
	s.Control <- "_CTL_CLEANUP"
}

func (s *Server) LogIn() {
	cmd := strings.Replace(s.Login, "%u", s.User, -1)
	cmd = strings.Replace(cmd, "%p", s.Pass, -1)

	s.Control <- cmd
}

func (s *Server) ReadIn(out chan<- string) {
	defer fmt.Println(s.path, ".ReadIn() Closing")
	for {
		infile, err := os.OpenFile(fmt.Sprint(s.path, "/in"), os.O_RDONLY, 0666)

		if err != nil {
			if os.IsNotExist(err) {
				return
			}
			fmt.Println(err)
			return
		}

		inbuf := bufio.NewReader(infile)

		if msg, err := inbuf.ReadString('\n'); err == nil {
			msg = msg[:len(msg)-1]

			if msg == "CTL_CLOSE" {
				infile.Close()
				out <- msg
				return
			}

			out <- msg
		}

		infile.Close()
	}
}

func (s *Server) WriteOut(in <-chan string) {
	defer fmt.Println(s.path, ".WriteOut() Closing")

	outfile, err := os.OpenFile(fmt.Sprint(s.path, "/out"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)

	if err != nil {
		fmt.Println(err)
		return
	}

	defer outfile.Close()

	outbuf := bufio.NewWriter(outfile)

	for {
		out, ok := <-in

		if !ok {
			_, _ = outbuf.WriteString("***CLOSED***\n")
			outbuf.Flush()
			return
		}

		ret, err := outbuf.WriteString(fmt.Sprintln(out))
		outbuf.Flush()

		if ret < len(out) && err != nil {
			fmt.Println(err)
			break
		}
	}
}

func (s *Server) Process() {
	defer fmt.Println(s.path, ".Process() Closing")

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
			if msg == "_CTL_CLEANUP" {
				close(snd)
				close(out)
				return
			}
			snd <- msg

		case msg := <-in:
			// Parse command messages here!
			if strings.HasPrefix(msg, "CTL_") {
				if msg == "CTL_CLOSE" {
					go s.Close()
					break
				}
				mainControl <- msg
				break
			}

			out <- msg
			snd <- msg

		case msg := <-rec:
			// Format output here!
			out <- msg
		}
	}
}

func (s *Server) Send(in <-chan string) {
	defer fmt.Println(s.path, ".Send() Closing")

	conbuf := bufio.NewWriter(s.netConn)

	for {
		msg, ok := <-in

		if !ok {
			conbuf.Flush()
			return
		}

		ret, err := conbuf.WriteString(fmt.Sprintln(msg))
		conbuf.Flush()

		if ret < len(msg) && err != nil {
			fmt.Println(err)
			break
		}
	}
}

func (s *Server) Recieve(out chan<- string) {
	defer fmt.Println(s.path, ".Recieve() Closing")

	conbuf := bufio.NewReader(s.netConn)

	for {
		if ret, err := conbuf.ReadString('\n'); err == nil {
			ret = ret[:len(ret)-1]

			out <- ret
		} else {
			break
		}
	}
}

var mainControl chan string = make(chan string, 1)

func readControl() {
	for {
		infile, err := os.OpenFile("in", os.O_RDONLY, 0666)

		if err != nil {
			if os.IsNotExist(err) {
				return
			}
			fmt.Println(err)
			return
		}

		inbuf := bufio.NewReader(infile)

		if msg, err := inbuf.ReadString('\n'); err == nil {
			msg = msg[:len(msg)-1]
			mainControl <- msg
		}

		infile.Close()
	}
}

func main() {
	if _, err := os.Stat("connections"); os.IsNotExist(err) {
		fmt.Println("connections directory does not exist; please see README.md")
		return
	}

	fmt.Println("macky - v0.1 - Starting Up")

	go ListHandler(sListAdd, sListDel)

	// Set up our superviser `in` FIFO
	for {
		if err := syscall.Mkfifo("in", syscall.S_IRWXU); err != nil {
			if os.IsExist(err) {
				if err := os.Remove("in"); err != nil {
					fmt.Println(err)
					return
				}
				continue
			}
			fmt.Println(err)
			return
		}
		break
	}

	defer func() {
		if err := os.Remove("in"); err != nil {
			fmt.Println(err)
		}
	}()

	defer CloseAllSessions() // Ensure we clean up

	go readControl()

	for {
		msg := <-mainControl
		fmt.Println("> ", msg)

		prs := strings.Split(msg, " ")

		switch prs[0] {
		case "CTL_QUIT":
			fmt.Println("Exiting, goodbye")
			return

		case "CTL_CONNECT":
			if len(prs) < 2 {
				fmt.Println("CTL_CONNECT: Missing arguments")
				break
			}
			for i := 1; i < len(prs); i++ {
				if SessionExists(prs[i]) {
					// TODO - Check if disconnected and if so, connect
					continue
				}

				s := new(Server)
				if err := s.ReadConf(prs[i]); err != nil {
					continue
				}
				if err := s.Connect(); err != nil {
					continue
				}
				s.Control = make(chan string)
				go s.Process()
				s.LogIn()
			}
		}
	}

	return
}
