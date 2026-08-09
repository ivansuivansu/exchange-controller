package telegram

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const callbackPrefix = "plan"

func EncodeCallback(planID string, version uint64, action Action) string {
	id := base64.RawURLEncoding.EncodeToString([]byte(planID))
	return fmt.Sprintf("%s:%s:%d:%s", callbackPrefix, id, version, action)
}

func DecodeCallback(data string) (planID string, version uint64, action Action, err error) {
	parts := strings.Split(data, ":")
	if len(parts) != 4 || parts[0] != callbackPrefix {
		return "", 0, "", errors.New("invalid callback payload")
	}
	id, decodeErr := base64.RawURLEncoding.DecodeString(parts[1])
	if decodeErr != nil || len(id) == 0 {
		return "", 0, "", errors.New("invalid callback plan ID")
	}
	version, err = strconv.ParseUint(parts[2], 10, 64)
	if err != nil || version == 0 {
		return "", 0, "", errors.New("invalid callback plan version")
	}
	action = Action(parts[3])
	if action != ActionApprove && action != ActionEdit && action != ActionReject {
		return "", 0, "", errors.New("invalid callback action")
	}
	return string(id), version, action, nil
}
