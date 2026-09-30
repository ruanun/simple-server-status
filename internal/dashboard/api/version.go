package api

import (
	"cmp"
	"strconv"
	"strings"
)

type semver struct {
	nums [3]int
	pre  []string
}

// parseSemver 解析正式发布版本号（可带 v 前缀与预发布后缀）
func parseSemver(v string) (semver, bool) {
	if !releaseVersionPattern.MatchString(v) {
		return semver{}, false
	}
	core, pre, _ := strings.Cut(strings.TrimPrefix(v, "v"), "-")
	var s semver
	for i, p := range strings.Split(core, ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			return semver{}, false
		}
		s.nums[i] = n
	}
	if pre != "" {
		s.pre = strings.Split(pre, ".")
	}
	return s, true
}

// comparePre 比较预发布标识：没有预发布标识的版本更大；数字标识按数值比较且小于字母标识
func comparePre(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		ai, aErr := strconv.Atoi(a[i])
		bi, bErr := strconv.Atoi(b[i])
		switch {
		case aErr == nil && bErr == nil:
			if c := cmp.Compare(ai, bi); c != 0 {
				return c
			}
		case aErr == nil:
			return -1
		case bErr == nil:
			return 1
		default:
			if c := strings.Compare(a[i], b[i]); c != 0 {
				return c
			}
		}
	}
	return cmp.Compare(len(a), len(b))
}

// compareSemver 按语义化版本比较：a < b 返回 -1，相等 0，a > b 返回 1；任一不是正式版本号时 ok 为 false
func compareSemver(a, b string) (int, bool) {
	va, okA := parseSemver(a)
	vb, okB := parseSemver(b)
	if !okA || !okB {
		return 0, false
	}
	for i := range va.nums {
		if c := cmp.Compare(va.nums[i], vb.nums[i]); c != 0 {
			return c, true
		}
	}
	return comparePre(va.pre, vb.pre), true
}

// agentOutdated Agent 版本低于 Dashboard 版本时返回 true；任一不是正式版本号时返回 false
func agentOutdated(agent, dashboard string) bool {
	c, ok := compareSemver(agent, dashboard)
	return ok && c < 0
}
