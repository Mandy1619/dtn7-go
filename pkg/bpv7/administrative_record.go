// SPDX-FileCopyrightText: 2019, 2020 Alvar Penning
// SPDX-FileCopyrightText: 2026 Markus Sommer
//
// SPDX-License-Identifier: GPL-3.0-or-later

package bpv7

import (
	"bytes"
	"fmt"
	"io"

	"github.com/dtn7/cboring"
)

type AdminRecordType uint64

// Sorted list of all known administrative record type codes to prevent double usage.
const (
	// AdminRecordTypeStatusReport is the administrative record type code for a status report.
	AdminRecordTypeStatusReport AdminRecordType = 1
)

// AdministrativeRecord describes an administrative record, e.g., a status report.
type AdministrativeRecord interface {
	cboring.CborMarshaler

	// RecordType returns this AdministrativeRecord's type code.
	RecordType() AdminRecordType
}

// WriteAdministrativeRecord writes record to a Writer.
func WriteAdministrativeRecord(ar AdministrativeRecord, w io.Writer) error {
	if err := cboring.WriteArrayLength(2, w); err != nil {
		return err
	}

	if err := cboring.WriteUInt(uint64(ar.RecordType()), w); err != nil {
		return err
	} else if err := cboring.Marshal(ar, w); err != nil {
		return NewMalformedAdminRecordError("error marshalling administrative record", err)
	}

	return nil
}

// ReadAdministrativeRecord from a Reader within its CBOR array and returns the wrapped data type.
func ReadAdministrativeRecord(r io.Reader) (AdministrativeRecord, error) {
	if n, err := cboring.ReadArrayLength(r); err != nil {
		return nil, err
	} else if n != 2 {
		return nil, NewMalformedAdminRecordError(fmt.Sprintf("expected CBOR array of length 2, got %d", n), nil)
	}

	rt, err := cboring.ReadUInt(r)
	if err != nil {
		return nil, err
	}

	recordType := AdminRecordType(rt)

	switch recordType {
	case AdminRecordTypeStatusReport:
		record := StatusReport{}
		err = cboring.Unmarshal(&record, r)
		if err != nil {
			return nil, err
		} else {
			return &record, nil
		}
	default:
		return nil, NewUnknownAdminRecordTypeError(recordType)
	}
}

// NewAdministrativeRecordFromCbor creates a new AdministrativeRecord from a given byte array.
func NewAdministrativeRecordFromCbor(data []byte) (ar AdministrativeRecord, err error) {
	buff := bytes.NewBuffer(data)
	return ReadAdministrativeRecord(buff)
}

// AdministrativeRecordToCbor creates a canonical block, containing this administrative record. The surrounding
// bundle _must_ have a set AdministrativeRecordPayload bundle processing control flag.
func AdministrativeRecordToCbor(ar AdministrativeRecord) (blk CanonicalBlock, err error) {
	buff := new(bytes.Buffer)
	if err = WriteAdministrativeRecord(ar, buff); err != nil {
		return
	}

	blk = NewCanonicalBlock(1, 0, NewPayloadBlock(buff.Bytes()))
	return
}
