package golib

import (
	"flag"
	"os"
	"runtime"
	"runtime/pprof"
)

var (
	// CpuProfileFile will be used to output a CPU profile of the running program,
	// if ProfileCpu() is called. This field is configured by the '-profile-cpu' flag
	// created by RegisterProfileFlags().
	CpuProfileFile = ""

	// MemProfileFile will be used to output a memory usage profile of the running program,
	// if ProfileCpu() is called. This field is configured by the '-profile-mem' flag
	// created by RegisterProfileFlags().
	MemProfileFile = ""
)

// RegisterProfileFlags registers flags to configure the CpuProfileFile and MemProfileFile
// by user-provided flags.
func RegisterProfileFlags() {
	flag.StringVar(&CpuProfileFile, "profile-cpu", CpuProfileFile, "Write cpu profile data to file.")
	flag.StringVar(&MemProfileFile, "profile-mem", MemProfileFile, "Write memory profile data to file.")
}

// ProfileCpu initiates memory and CPU profiling if any of the CpuProfileFile and MemProfileFile
// is set to non-empty strings, respectively. The function returns a tear-down function
// that must be called before the program exists in order to flush the profiling data to the
// output files.
// It can be used like this:
//   defer golib.ProfileCpu()()
func ProfileCpu() func() {
	var cpu, mem *os.File
	var err error
	if CpuProfileFile != "" {
		cpu, err = os.Create(CpuProfileFile)
		if err != nil {
			Log.Fatalln(err)
		}
		if err := pprof.StartCPUProfile(cpu); err != nil {
			Log.Fatalln("Unable to start CPU profile:", err)
		}
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
			runtime.GC() // get up-to-date statistics
			if err := pprof.WriteHeapProfile(mem); err != nil {
				Log.Warnln("Failed to write Memory profile:", err)
			}
			mem.Close()
		}
	}
}
