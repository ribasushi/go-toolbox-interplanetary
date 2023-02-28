package fil //nolint:revive

import (
	"context"
	"net/http"
	"time"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/api/v0api"
	lotustypes "github.com/filecoin-project/lotus/chain/types"
)

type (
	LotusAPIClient   = v0api.FullNodeStruct
	LotusBeaconEntry = lotustypes.BeaconEntry
	LotusTS          = lotustypes.TipSet
	LotusTSK         = lotustypes.TipSetKey
)

func LotusAPIClientV0(ctx context.Context, url string, timeoutSecs int, bearerToken string) (*LotusAPIClient, jsonrpc.ClientCloser, error) { //nolint:revive
	if timeoutSecs == 0 {
		timeoutSecs = 30
	}
	hdr := make(http.Header, 1)
	if bearerToken != "" {
		hdr["Authorization"] = []string{"Bearer " + bearerToken}
	}
	c := new(v0api.FullNodeStruct)
	closer, err := jsonrpc.NewMergeClient(
		ctx,
		url+"/rpc/v0",
		"Filecoin",
		[]interface{}{&c.Internal, &c.CommonStruct.Internal},
		hdr,
		jsonrpc.WithTimeout(time.Duration(timeoutSecs)*time.Second),
	)
	if err != nil {
		return nil, nil, err
	}
	return c, closer, nil
}
