package coretools

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyz"
const (
	letterIdxBits = 5                    // 5 bits to represent a letter index for 26 letters
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var src = rand.NewSource(time.Now().UnixNano())

// from https://stackoverflow.com/a/31832326
func RandomString(n int) string {
	sb := strings.Builder{}
	sb.Grow(n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			sb.WriteByte(letterBytes[idx])
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return sb.String()
}

// RobotConfigNamespace constructs the namespace name where tenants robot config is situated
func RobotConfigNamespace(tenantMainNamespace string) string {
	if tenantMainNamespace == "default" {
		return RobotConfigCloudNamespace
	}
	return fmt.Sprintf("%s-%s", tenantMainNamespace, RobotConfigCloudNamespace)
}

// TenantMainNamespace constructs the main namespace of a tenant where all its CRs are stored
func TenantMainNamespace(tenantName string) string {
	if tenantName == DefaultTenantName {
		return "default"
	}
	return fmt.Sprintf("%s%s", TenantPrefix, tenantName)
}
