package chunker

// gearTable holds 256 pseudo-random 64-bit constants used by the gear hash
// in cutPoint. It is generated once at init time via SplitMix64 from a fixed
// seed, so the table (and therefore the chunk boundaries for any given
// input) is stable across runs and builds.
var gearTable = generateGearTable()

func generateGearTable() [256]uint64 {
	var table [256]uint64
	var seed uint64 = 0x9E3779B97F4A7C15
	for i := range table {
		seed += 0x9E3779B97F4A7C15
		z := seed
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		z = z ^ (z >> 31)
		table[i] = z
	}
	return table
}
