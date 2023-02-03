package offline

import (
	"os"
)

type DiskStatus struct {
	All  uint64 `json:"all"`
	Used uint64 `json:"used"`
	Free uint64 `json:"free"`
}

type CompressImage struct {
}

// FreeDiskSpace 返回磁盘剩余空间 - MB Size
//func (cs *CompressImage) FreeDiskSpace(driverID string) uint64 {
//	diskUsage := func(driverID string) uint64 {
//		fs := syscall.Statfs_t{}
//		err := syscall.Statfs(driverID, &fs)
//		if err != nil {
//			log.Printf("init driver syscall error : %s", err)
//			return 0
//		}
//		return fs.Bfree * uint64(fs.Bsize)
//	}
//	return diskUsage(driverID) / bytes.MB
//}

func (cs *CompressImage) Compress() {

}

func (cs *CompressImage) UnCompress() {

}

func (cs *CompressImage) SaveOf(sPath string) *os.File {
	return nil
}
