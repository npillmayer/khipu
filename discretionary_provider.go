package khipu

import (
	"fmt"
	"github.com/npillmayer/khipu/contracts"
)

func replacementParameters(run preparedRun, s runSourceRange) (runParameters, error) {
	params := run.params
	features, err := intersectFeatures(params.shaping.Features, run.source.scalarEnd-run.source.scalarStart,
		s.scalarStart-run.source.scalarStart, s.scalarEnd-run.source.scalarStart)
	if err != nil {
		return runParameters{}, err
	}
	for i := range features {
		if features[i].Start != 0 || uint64(features[i].End) != s.scalarEnd-s.scalarStart {
			return runParameters{}, fmt.Errorf("%w: replacement has range-limited features", contracts.ErrIllegalArguments)
		}
		// A feature covering the entire original box applies to both complete
		// fragments. Interior feature boundaries cannot be guessed after edits.
		features[i].Start, features[i].End = 0, 0
	}
	params.shaping.Features = features
	return params, nil
}
