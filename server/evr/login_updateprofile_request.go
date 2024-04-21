package evr

import (
	"encoding/binary"
	"fmt"

	"github.com/gofrs/uuid/v5"
)

type UpdateClientProfile struct {
	Session       uuid.UUID
	EvrId         EvrId
	ClientProfile ClientProfile
}

func (m *UpdateClientProfile) Token() string {
	return "SNSUpdateProfile"
}

func (m *UpdateClientProfile) Symbol() Symbol {
	return ToSymbol(m.Token())
}

func (lr *UpdateClientProfile) String() string {
	return fmt.Sprintf("UpdateProfile(session=%s, evr_id=%s)", lr.Session.String(), lr.EvrId.String())
}

func (m *UpdateClientProfile) Stream(s *EasyStream) error {
	return RunErrorFunctions([]func() error{
		func() error { return s.StreamGuid(&m.Session) },
		func() error { return s.StreamNumber(binary.LittleEndian, &m.EvrId.PlatformCode) },
		func() error { return s.StreamNumber(binary.LittleEndian, &m.EvrId.AccountId) },
		func() error { return s.StreamJson(&m.ClientProfile, true, NoCompression) },
	})
}
