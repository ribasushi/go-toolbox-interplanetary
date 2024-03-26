package fil //nolint:revive

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/api/v0api"
	lotustypes "github.com/filecoin-project/lotus/chain/types"
)

type (
	LotusDaemonAPIClientV0 = v0api.FullNode
	LotusMinerAPIClientV0  = v0api.StorageMiner
	LotusBeaconEntry       = lotustypes.BeaconEntry
	LotusTS                = lotustypes.TipSet
	LotusTSK               = lotustypes.TipSetKey
)

var hasV0Suffix = regexp.MustCompile(`\/rpc\/v0\/?\z`)

func NewLotusDaemonAPIClientV0(ctx context.Context, url string, timeoutSecs int, bearerToken string) (LotusDaemonAPIClientV0, jsonrpc.ClientCloser, error) {
	if timeoutSecs == 0 {
		timeoutSecs = 30
	}
	hdr := make(http.Header, 1)
	if bearerToken != "" {
		hdr["Authorization"] = []string{"Bearer " + bearerToken}
	}

	if !hasV0Suffix.MatchString(url) {
		url += "/rpc/v0"
	}

	c := new(v0api.FullNodeStruct)
	closer, err := jsonrpc.NewMergeClient(
		ctx,
		url,
		"Filecoin",
		[]interface{}{&c.Internal, &c.CommonStruct.Internal},
		hdr,
		// deliberately do not use jsonrpc.WithErrors(api.RPCErrors)
		jsonrpc.WithTimeout(time.Duration(timeoutSecs)*time.Second),
	)
	if err != nil {
		return nil, nil, err
	}
	return c, closer, nil
}

func NewLotusMinerAPIClientV0(ctx context.Context, url string, timeoutSecs int, bearerToken string) (LotusMinerAPIClientV0, jsonrpc.ClientCloser, error) { //nolint:revive
	if timeoutSecs == 0 {
		timeoutSecs = 30
	}
	hdr := make(http.Header, 1)
	if bearerToken != "" {
		hdr["Authorization"] = []string{"Bearer " + bearerToken}
	}

	if !hasV0Suffix.MatchString(url) {
		url += "/rpc/v0"
	}
	return client.NewStorageMinerRPCV0(
		ctx,
		url,
		hdr,
		jsonrpc.WithTimeout(time.Duration(timeoutSecs)*time.Second),
	)
}
