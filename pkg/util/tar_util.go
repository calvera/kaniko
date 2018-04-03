/*
Copyright 2018 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"archive/tar"
	"compress/gzip"
	pkgutil "github.com/GoogleCloudPlatform/container-diff/pkg/util"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"syscall"
)

var hardlinks = make(map[uint64]string)

// AddToTar adds the file i to tar w at path p
func AddToTar(p string, i os.FileInfo, w *tar.Writer) error {
	linkDst := ""
	if i.Mode()&os.ModeSymlink != 0 {
		var err error
		linkDst, err = os.Readlink(p)
		if err != nil {
			return err
		}
	}
	hdr, err := tar.FileInfoHeader(i, linkDst)
	if err != nil {
		return err
	}
	hdr.Name = p

	hardlink, linkDst := checkHardlink(p, i)
	if hardlink {
		hdr.Linkname = linkDst
		hdr.Typeflag = tar.TypeLink
		hdr.Size = 0
	}
	if err := w.WriteHeader(hdr); err != nil {
		return err
	}
	if !(i.Mode().IsRegular()) || hardlink {
		return nil
	}
	r, err := os.Open(p)
	if err != nil {
		return err
	}
	defer r.Close()
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return nil
}

// Returns true if path is hardlink, and the link destination
func checkHardlink(p string, i os.FileInfo) (bool, string) {
	hardlink := false
	linkDst := ""
	if sys := i.Sys(); sys != nil {
		if stat, ok := sys.(*syscall.Stat_t); ok {
			nlinks := stat.Nlink
			if nlinks > 1 {
				inode := stat.Ino
				if original, exists := hardlinks[inode]; exists && original != p {
					hardlink = true
					logrus.Debugf("%s inode exists in hardlinks map, linking to %s", p, original)
					linkDst = original
				} else {
					hardlinks[inode] = p
				}
			}
		}
	}
	return hardlink, linkDst
}

// UnpackCompressedTar unpacks the compressed tar at path to dir
func UnpackCompressedTar(path, dir string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()
	return pkgutil.UnTar(gzr, dir, nil)
}
