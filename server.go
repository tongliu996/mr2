// Copyright (c) 2019-present Cloud <cloud@txthinking.com>
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of version 3 of the GNU General Public
// License as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"errors"
	"log"
	"net"
	"strconv"

	"github.com/gogo/protobuf/proto"
	cache "github.com/patrickmn/go-cache"
	"github.com/txthinking/x"
)

// Server .
type Server struct {
	TCPAddr   *net.TCPAddr
	UDPAddr   *net.UDPAddr
	TCPListen *net.TCPListener
	UDPConn   *net.UDPConn
	Cache     *cache.Cache
	Ckv       *x.CryptKV
}

// NewServer .
func NewServer(addr, password string) (*Server, error) {
	taddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	uaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	s := &Server{
		TCPAddr: taddr,
		UDPAddr: uaddr,
		Cache:   cache.New(cache.NoExpiration, cache.NoExpiration),
		Ckv: &x.CryptKV{
			AESKey: []byte(password),
		},
	}
	return s, nil
}

// ListenAndServe .
func (s *Server) ListenAndServe() error {
	errch := make(chan error)
	go func() {
		errch <- s.RunTCPServer()
	}()
	go func() {
		errch <- s.RunUDPServer()
	}()
	return <-errch
}

// RunTCPServer
func (s *Server) RunTCPServer() error {
	var err error
	s.TCPListen, err = net.ListenTCP("tcp", s.TCPAddr)
	if err != nil {
		return err
	}
	defer s.TCPListen.Close()
	for {
		c, err := s.TCPListen.AcceptTCP()
		if err != nil {
			return err
		}
		go func(c *net.TCPConn) {
			if err := s.TCPHandle(c); err != nil {
				log.Println(err)
			}
		}(c)
	}
	return nil
}

// TCPHandle
func (s *Server) TCPHandle(c *net.TCPConn) error {
	defer c.Close()
	t, err := NewTCPServer(s, c)
	if err != nil {
		return err
	}
	if err := t.ListenAndServe(); err != nil {
		return err
	}
	return nil
}

// RunUDPServer
func (s *Server) RunUDPServer() error {
	var err error
	s.UDPConn, err = net.ListenUDP("udp", s.UDPAddr)
	if err != nil {
		return err
	}
	defer s.UDPConn.Close()
	for {
		b := make([]byte, 65536)
		n, addr, err := s.UDPConn.ReadFromUDP(b)
		if err != nil {
			return err
		}
		go func(addr *net.UDPAddr, b []byte) {
			if err := s.UDPHandle(addr, b); err != nil {
				log.Println(err)
			}
		}(addr, b[0:n])
	}
	return nil
}

// UDPHandle
func (s *Server) UDPHandle(addr *net.UDPAddr, b []byte) error {
	p := &UDPPacket{}
	if err := proto.Unmarshal(b, p); err != nil {
		return err
	}
	if p.Address == "" {
		tmp, err := s.Ckv.Decrypt(p.Key, "Mr.2", 3*60)
		if err != nil || tmp != "UDPPacket" {
			return errors.New("Hacking")
		}
		u, err := NewUDPServer(s, p.Port, addr)
		if err != nil {
			return err
		}
		s.Cache.Set("u:"+strconv.FormatInt(p.Port, 10), u, cache.DefaultExpiration)
		defer s.Cache.Delete("u:" + strconv.FormatInt(p.Port, 10))
		if err := u.ListenAndServe(); err != nil {
			return err
		}
		return nil
	}
	i, ok := s.Cache.Get("u:" + strconv.FormatInt(p.Port, 10))
	if !ok {
		return nil
	}
	c := i.(*UDPServer)
	if err := c.HandlePacket(p); err != nil {
		return err
	}
	return nil
}
