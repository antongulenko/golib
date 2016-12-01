package golib

import (
	"flag"
	"os"
	"runtime/pprof"
)

var (
	CpuProfileFile = ""
	MemProfileFile = ""
)

func init() {
	flag.StringVar(&CpuProfileFile, "cpuprofile", CpuProfileFile, "Write cpu profile data to file")
	flag.StringVar(&MemProfileFile, "memprofile", MemProfileFile, "Write memory profile data to file")
}

// Usage: defer golib.ProfileCpu()()
// Performs both cpu and memory profiling, if enabled
func ProfileCpu() (stopProfiling func()) {
	var cpu, mem *os.File
	var err error
	if CpuProfileFile != "" {
		cpu, err = os.Create(CpuProfileFile)
		if err != nil {
			Log.Fatalln(err)
		}
		pprof.StartCPUProfile(cpu)
	}
	if MemProfileFile != "" {
		mem, err = os.Create(MemProfileFile)
		if err != nil {
			Log.Fatalln(err)
		}
	}
	return func() {
		if cpu != nil {
			pprof.StopCPUProfile()
		}
		if mem != nil {
			pprof.WriteHeapProfile(mem)
			mem.Close()
		}
	}
}
