package main

import (
	"cjones.org/hg/go-xmpp2.hg/xmpp"
	"log"
	"time"
)

type Pinger struct {
	*time.Ticker
	*time.Timer
	timerRunning bool
	stop         chan bool
	timeout      chan bool
}

func MakePinger(target xmpp.JID, period int, timeout int,
	action func(xmpp.JID) bool) *Pinger {
	p := &Pinger{
		Ticker:       time.NewTicker(time.Duration(period) * time.Second),
		timerRunning: false,
		stop:         make(chan bool),
		timeout:      make(chan bool),
	}
	go func(p *Pinger) {
		defer func() {
			p.Ticker.Stop()
		}()
		defer close(p.stop)

		for {
			select {
			case <-p.Ticker.C:
				log.Printf("ping to %s", target)
				m := &xmpp.Iq{
					Header: xmpp.Header{
						To:       target,
						From:     Client.Jid,
						Id:       xmpp.NextId(),
						Type:     "get",
						Innerxml: "<ping xmlns='urn:xmpp:ping'/>",
					},
				}
				Client.Send <- m
				if !p.timerRunning {
					if p.Timer == nil {
						p.Timer = time.AfterFunc(time.Duration(timeout)*time.Second,
							func() {
								p.timeout <- true
							})
						defer p.Timer.Stop()
					} else {
						p.Timer.Reset(time.Duration(timeout) * time.Second)
					}
					p.timerRunning = true
				}
				cb := func(s xmpp.Stanza) bool {
					iq, ok := s.(*xmpp.Iq)
					if !ok {
						return true
					}
					log.Print("ping reply type: ", iq.Type)
					p.Timer.Stop()
					p.timerRunning = false
					return false
				}
				Client.SetCallback(m.Id, cb)
			case <-p.timeout:
				log.Printf("pinger for %s timed out", target)
				if action == nil || !action(target) { // if no action is set, just stop
					p.stop <- true
				}
			case <-p.stop:
				log.Printf("pinger stop requested")
				return
			}
		}
	}(p)
	return p
}

func (p *Pinger) Stop() {
	if p.stop != nil {
		p.stop <- true
	}
}
