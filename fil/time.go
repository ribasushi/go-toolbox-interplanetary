package fil //nolint:revive

import (
	"context"
	"time"

	filabi "github.com/filecoin-project/go-state-types/abi"
	filbuiltin "github.com/filecoin-project/go-state-types/builtin"
	lotusapi "github.com/filecoin-project/lotus/api"
	lotusbuild "github.com/filecoin-project/lotus/build"
	lotustypes "github.com/filecoin-project/lotus/chain/types"
	"golang.org/x/xerrors"
)

const FilGenesisUnix = 1598306400 //nolint:revive

var APIMaxTipsetsBehind = uint64(4) // APIMaxTipsetsBehind should not be set too low: a nul tipset is indistinguishable from loss of sync

func MainnetTime(e filabi.ChainEpoch) time.Time { return time.Unix(int64(e)*30+FilGenesisUnix, 0) } //nolint:revive

func WallTimeEpoch(t time.Time) filabi.ChainEpoch { //nolint:revive
	return filabi.ChainEpoch(t.Unix()-FilGenesisUnix) / filbuiltin.EpochDurationSeconds
}

func GetTipset(ctx context.Context, lapi *lotusapi.FullNodeStruct, lookback filabi.ChainEpoch) (*lotustypes.TipSet, error) { //nolint:revive
	latestHead, err := lapi.ChainHead(ctx)
	if err != nil {
		return nil, xerrors.Errorf("failed getting chain head: %w", err)
	}

	wallUnix := time.Now().Unix()
	filUnix := int64(latestHead.Blocks()[0].Timestamp)

	if wallUnix < filUnix-2 || // allow couple seconds clock-drift tolerance
		wallUnix > filUnix+int64(
			lotusbuild.PropagationDelaySecs+(APIMaxTipsetsBehind*filbuiltin.EpochDurationSeconds),
		) {
		return nil, xerrors.Errorf(
			"lotus API out of sync: chainHead reports unixtime %d (height: %d) while walltime is %d (delta: %s)",
			filUnix,
			latestHead.Height(),
			wallUnix,
			time.Second*time.Duration(wallUnix-filUnix),
		)
	}

	if lookback == 0 {
		return latestHead, nil
	}

	latestHeight := latestHead.Height()
	tipsetAtLookback, err := lapi.ChainGetTipSetByHeight(ctx, latestHeight-lookback, latestHead.Key())
	if err != nil {
		return nil, xerrors.Errorf("determining target tipset %d epochs ago failed: %w", lookback, err)
	}

	return tipsetAtLookback, nil
}
