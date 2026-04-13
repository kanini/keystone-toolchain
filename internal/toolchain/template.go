package toolchain

import (
	"fmt"
	"strings"

	"github.com/kanini/keystone-toolchain/internal/contract"
)

type templateVars struct {
	stageBin       string
	stageBinParent string
}

func expandCommandArgs(args []string, vars templateVars) ([]string, *contract.AppError) {
	expanded := make([]string, 0, len(args))
	for _, arg := range args {
		value := strings.ReplaceAll(arg, "{{stage_bin}}", vars.stageBin)
		value = strings.ReplaceAll(value, "{{stage_bin_parent}}", vars.stageBinParent)
		if strings.Contains(value, "{{") || strings.Contains(value, "}}") {
			return nil, contract.Validation(contract.CodeConfigInvalid, fmt.Sprintf("Unsupported template token in command argument %q.", arg), "Fix the embedded adapter manifest.")
		}
		expanded = append(expanded, value)
	}
	return expanded, nil
}
