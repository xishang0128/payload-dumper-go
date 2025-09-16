package dumper

import (
	"encoding/binary"
	"fmt"

	"github.com/xishang0128/payload-dumper-go/common/i18n"
	"github.com/xishang0128/payload-dumper-go/common/metadata"
	"github.com/xishang0128/payload-dumper-go/common/ziputil"

	"google.golang.org/protobuf/proto"
)

// ExtractMetadata extracts and returns the metadata from the payload
func (d *Dumper) ExtractMetadata() ([]byte, error) {
	path := "META-INF/com/android/metadata"
	offset, size, err := ziputil.GetStoredEntryOffset(d.payloadFile, path)
	if err != nil {
		return nil, fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToExtractMetadata, path, err)
	}

	data, err := d.payloadFile.Read(offset, int(size))
	if err != nil {
		return nil, fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToReadMetadata, err)
	}

	return data, nil
}

func (d *Dumper) parseMetadata() error {
	headLen := 4 + 8 + 8 + 4
	buf, err := d.payloadFile.Read(d.baseOffset, headLen)
	if err != nil {
		return err
	}

	if len(buf) != headLen {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorInsufficientDataForHeader)
	}

	magic := string(buf[:4])
	if magic != "CrAU" {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorInvalidMagic, magic)
	}

	ver := binary.BigEndian.Uint64(buf[4:12])
	if ver != 2 {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorUnsupportedFileFormat, ver)
	}

	maniSize := binary.BigEndian.Uint64(buf[12:20])
	sigSize := uint32(0)

	if ver > 1 {
		sigSize = binary.BigEndian.Uint32(buf[20:24])
	}

	fp := d.baseOffset + int64(headLen)

	maniData, err := d.payloadFile.Read(fp, int(maniSize))
	if err != nil {
		return err
	}
	fp += int64(maniSize)

	fp += int64(sigSize)

	d.dataOffset = fp
	d.manifest = &metadata.DeltaArchiveManifest{}

	if err := proto.Unmarshal(maniData, d.manifest); err != nil {
		return fmt.Errorf(i18n.I18nMsg.Dumper.ErrorFailedToParseManifest, err)
	}

	d.blockSize = d.manifest.GetBlockSize()
	if d.blockSize == 0 {
		d.blockSize = 4096
	}

	return nil
}
