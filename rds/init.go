package rds

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/sirupsen/logrus"
)

var (
	rdsConn     quic.Connection
	timeToRetry = 10
)

func init() {
	go connect(true)
}

func connect(inital bool) {
	var err error
	rdsConn, err = quic.DialAddr(context.Background(), "oomph.ethaniccc.tech:5435", &tls.Config{
		ServerName: "oomph-rds-ethaniccc",
		NextProtos: []string{"oomph-rds-v0.0"},
	}, &quic.Config{
		Versions:              []quic.Version{quic.Version2},
		KeepAlivePeriod:       time.Second,
		MaxIdleTimeout:        time.Minute,
		MaxIncomingStreams:    4096,
		MaxIncomingUniStreams: 0,
		EnableDatagrams:       false,
	})

	if err != nil {
		rdsConn = nil
		timeToRetry *= 2
		if timeToRetry > 120 {
			timeToRetry = 120
		}

		logrus.Warnf("Failed to connect to the RDS, retrying in %d seconds", timeToRetry)
		time.Sleep(time.Duration(timeToRetry) * time.Second)

		// Retry the connection again.
		connect(false)
		return
	}

	if inital {
		logrus.Info("Connected to the RDS")
	} else {
		logrus.Info("Re-established connection to the RDS")
	}
	timeToRetry = 2
}

func Available() bool {
	return rdsConn != nil
}
