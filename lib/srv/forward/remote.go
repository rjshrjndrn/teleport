/*
Copyright 2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package forward

import (
	"net"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// newRemoteSession will create and return a *ssh.Client and *ssh.Session
// with a remote host.
func newRemoteSession(dstAddr string, systemLogin string, userAgent agent.Agent, authHandlers *srv.AuthHandlers) (*ssh.Client, *ssh.Session, error) {
	// the proxy will use the agent that has been forwarded to it as the auth
	// method when connecting to the remote host
	if userAgent == nil {
		return nil, nil, trace.AccessDenied("agent must be forwarded to proxy")
	}
	authMethod := ssh.PublicKeysCallback(userAgent.Signers)

	clientConfig := &ssh.ClientConfig{
		User: systemLogin,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: authHandlers.HostKeyAuth,
		Timeout:         defaults.DefaultDialTimeout,
	}

	// dial with a timeout
	client, err := dialTimeout("tcp", dstAddr, clientConfig)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return client, session, nil
}

// dialTimeout will both Dial (with a timeout) as well as place a timeout on
// read/write on the underlying net.Conn.
func dialTimeout(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	conn, err := net.DialTimeout(network, addr, config.Timeout)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// wrap the conn in a utils.TimeoutConn so we can set a read and write deadline
	timeoutConn := utils.ObeyIdleTimeout(conn, defaults.DefaultIdleConnectionDuration, "forward-node-upstream")

	c, chans, reqs, err := ssh.NewClientConn(timeoutConn, addr, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ssh.NewClient(c, chans, reqs), nil
}
