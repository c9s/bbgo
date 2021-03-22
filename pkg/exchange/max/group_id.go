package max

import "hash/fnv"

func GenerateGroupID(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
