package lp2p //nolint:revive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	cborutil "github.com/filecoin-project/go-cbor-util"
	lp2p "github.com/libp2p/go-libp2p"
	lp2phost "github.com/libp2p/go-libp2p/core/host"
	lp2pnet "github.com/libp2p/go-libp2p/core/network"
	lp2ppeer "github.com/libp2p/go-libp2p/core/peer"
	lp2pproto "github.com/libp2p/go-libp2p/core/protocol"
	lp2pyamux "github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	lp2pconnmgr "github.com/libp2p/go-libp2p/p2p/net/connmgr"
	lp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	lp2ptcp "github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/libp2p/go-yamux/v4"
	"github.com/multiformats/go-multiaddr"
	infomempeerstore "github.com/ribasushi/go-libp2p-infomempeerstore"
	"github.com/ribasushi/go-toolbox/cmn"
)

type Host = lp2phost.Host
type PeerID = lp2ppeer.ID

// AddrInfo is a better annotated version of lp2ppeer.AddrInfo
type AddrInfo struct {
	PeerID     *PeerID               `json:"peerid"`
	MultiAddrs []multiaddr.Multiaddr `json:"multiaddrs"`
}

func (ai *AddrInfo) ToLp2p() lp2ppeer.AddrInfo {
	return lp2ppeer.AddrInfo{
		ID:    *ai.PeerID,
		Addrs: ai.MultiAddrs,
	}
}

func AddrInfoFromLp2p(ai lp2ppeer.AddrInfo) AddrInfo {
	return AddrInfo{
		PeerID:     &ai.ID,
		MultiAddrs: ai.Addrs,
	}
}

func NewPlainNodeTCP(withTimeout time.Duration) (lp2phost.Host, *infomempeerstore.PeerStore, error) {
	ps, err := infomempeerstore.NewPeerstore()
	if err != nil {
		return nil, nil, cmn.WrErr(err)
	}

	connmgr, err := lp2pconnmgr.NewConnManager(8192, 16384) // effectively deactivate
	if err != nil {
		return nil, nil, cmn.WrErr(err)
	}

	yc := yamux.DefaultConfig()
	yc.EnableKeepAlive = true
	yc.KeepAliveInterval = 10 * time.Second
	yc.MeasureRTTInterval = 10 * time.Second
	yc.ConnectionWriteTimeout = 5 * time.Second
	// rest of default settings from https://github.com/libp2p/go-libp2p/blob/v0.27.3/p2p/muxer/yamux/transport.go#L17-L34
	yc.MaxStreamWindowSize = uint32(32 << 20)
	yc.LogOutput = io.Discard
	yc.ReadBufSize = 0
	yc.MaxIncomingStreams = math.MaxUint32

	nodeHost, err := lp2p.New(
		lp2p.Peerstore(ps),  // allows us collect random on-connect data
		lp2p.RandomIdentity, // *NEVER* reuse a peerid
		lp2p.ConnectionManager(connmgr),
		lp2p.ResourceManager(&lp2pnet.NullResourceManager{}),
		lp2p.Ping(false),
		lp2p.DisableMetrics(),
		lp2p.DisableRelay(),
		lp2p.NoListenAddrs,
		lp2p.NoTransports,
		lp2p.Transport(lp2ptcp.NewTCPTransport, lp2ptcp.WithConnectionTimeout(withTimeout+100*time.Millisecond)),
		lp2p.Security(lp2ptls.ID, lp2ptls.New),
		lp2p.UserAgent("interplanetary-toolbox"),
		lp2p.WithDialTimeout(withTimeout),
		lp2p.Muxer(lp2pyamux.ID, (*lp2pyamux.Transport)(yc)),
	)
	if err != nil {
		return nil, nil, cmn.WrErr(err)
	}

	return nodeHost, ps, nil
}

var DefaultRPCTimeout = time.Duration(5 * time.Minute)

type RPCTook struct {
	LocalPeerID      *PeerID `json:"dialing_peerid"`
	PeerConnectMsecs *int64  `json:"peer_connect_took_msecs,omitempty"`
	StreamOpenMsecs  *int64  `json:"stream_open_took_msecs,omitempty"`
	StreamWriteMsecs *int64  `json:"stream_write_took_msecs,omitempty"`
	StreamReadMsecs  *int64  `json:"stream_read_took_msecs,omitempty"`
}

func ExecCborRPC(
	ctx context.Context, nodeHost Host,
	peerAddr AddrInfo, proto lp2pproto.ID,
	args interface{}, resp interface{},
) (RPCTook, error) {

	myID := nodeHost.ID()
	t := RPCTook{LocalPeerID: &myID}

	unprot, connectTook, err := ConnectAndProtect(ctx, nodeHost, peerAddr)
	t.PeerConnectMsecs = &connectTook
	if err != nil {
		return t, err
	}
	defer unprot()

	t0 := time.Now()
	st, err := nodeHost.NewStream(ctx, *peerAddr.PeerID, proto)
	t1 := time.Since(t0).Milliseconds()
	t.StreamOpenMsecs = &t1
	if err != nil {
		return t, fmt.Errorf("error while opening %s stream: %w", proto, err)
	}
	defer st.Close() //nolint:errcheck

	// inherit deadline from the context due to the bizarere duality of needing both
	// - a context for NewStream
	// - a clock-deadline for the read/write
	// if unavailable - do an absurdly long DefaultRPCTimeout
	dline, hasDline := ctx.Deadline()
	if !hasDline {
		dline = time.Now().Add(DefaultRPCTimeout)
	}
	st.SetDeadline(dline)             //nolint:errcheck
	defer st.SetDeadline(time.Time{}) //nolint:errcheck

	if args != nil {
		t0 = time.Now()
		err = cborutil.WriteCborRPC(st, args)
		t2 := time.Since(t0).Milliseconds()
		t.StreamWriteMsecs = &t2

		if err != nil {
			return t, fmt.Errorf("error while writing to %s stream: %w", proto, err)
		}
	}

	t0 = time.Now()
	err = cborutil.ReadCborRPC(st, resp)
	t3 := time.Since(t0).Milliseconds()
	t.StreamReadMsecs = &t3

	if err != nil {
		return t, fmt.Errorf("error while reading %s response: %w", proto, err)
	}

	return t, nil
}

func ConnectAndProtect(ctx context.Context, nodeHost Host, ai AddrInfo) (closer func(), tookMsecs int64, err error) {
	t0 := time.Now()

	protTag := fmt.Sprintf("conn-%s-%d", ai.PeerID.String(), t0.UnixNano())
	nodeHost.ConnManager().Protect(*ai.PeerID, protTag)
	closer = func() {
		nodeHost.ConnManager().Unprotect(*ai.PeerID, protTag)
	}

	err = nodeHost.Connect(ctx, ai.ToLp2p())
	tookMsecs = time.Since(t0).Milliseconds()

	if err != nil {
		// unprotect right away on error
		closer()
		closer = func() {}
		err = fmt.Errorf("error ensuring connection to %s: %w", ai.PeerID.String(), err)
	}

	return
}

func AssembleAddrInfo[M interface{ ~string | ~[]byte }](pID *PeerID, addrs []M) (AddrInfo, error) {

	var ai AddrInfo
	errs := make([]error, 0, 4)

	if pID == nil || *pID == "" {
		errs = append(errs, errors.New("no peerID supplied"))
	} else {
		decID, err := lp2ppeer.Decode(pID.String())
		if err != nil {
			errs = append(errs, fmt.Errorf("peerID '%s' is not valid: %w", pID.String(), err))
		} else {
			ai.PeerID = &decID
		}
	}

	maddrs := make([]multiaddr.Multiaddr, 0, len(addrs))
	var err error
	var ma multiaddr.Multiaddr
	for i, encMA := range addrs {

		switch any(encMA).(type) {
		case string:
			ma, err = multiaddr.NewMultiaddr(string(encMA))
		default:
			ma, err = multiaddr.NewMultiaddrBytes([]byte(encMA))
		}

		if err != nil {
			errs = append(errs, fmt.Errorf("multiaddress entry '%x' (#%d) is not valid: %w", encMA, i, err))
		} else {
			maddrs = append(maddrs, ma)
		}
	}

	if len(errs) > 0 {
		return ai, errors.Join(errs...)
	}

	ai.MultiAddrs = maddrs
	return ai, nil
}
