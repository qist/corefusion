package portal

import (
	"strings"
	"testing"
)

// 存量模板（未打过补丁）：应补上 longSubUrl、短链优先、严格匹配与长链接兜底。
func TestPatchPortalJSShortLinks(t *testing.T) {
	old := "    var subUrl = computed(function() {\n" +
		"      if (!client.value.sub_token) return ''\n" +
		"      var t = client.value.sub_token\n" +
		"      return window.location.origin + '/sub/' + t\n" +
		"    })\n" +
		"    var subInsecureHidden = computed(function() {\n" +
		"      return !!client.value.sub_token && !subUrl.value\n" +
		"    })\n\n" +
		"var buildSubLink = function(formatKey, configName, platform, providers) {\n" +
		"  var base = subUrl.value\n" +
		"  if (!base) return ''\n" +
		"  return base\n" +
		"}\n"

	got := patchPortalJSShortLinks(old)
	for _, want := range []string{
		"var longSubUrl = computed",
		"client.value.sub_short_url",
		"findShortLink",
		"var wantConfig = configName || ''",
		"var short = findShortLink(formatKey, configName, platform, providers)",
		"var base = longSubUrl.value",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("补丁后缺少 %q", want)
		}
	}

	// 幂等：已打过补丁的模板不能重复插入
	if again := patchPortalJSShortLinks(got); again != got {
		t.Fatal("patch 不幂等")
	}
}

// 上一版补丁（宽松匹配 + 短链再拼参数）应被就地升级。
func TestPatchPortalJSShortLinksUpgradesLooseVersion(t *testing.T) {
	old := "    var subUrl = computed(function() {\n" +
		"      if (!client.value.sub_token) return ''\n" +
		"      if (client.value.sub_short_url) return client.value.sub_short_url\n" +
		"      var t = client.value.sub_token\n" +
		"      return window.location.origin + '/sub/' + t\n" +
		"    })\n" +
		"    var subInsecureHidden = computed(function() {\n" +
		"      return false\n" +
		"    })\n\n" +
		"var findShortLink = function(formatKey, configName, platform, providers) {\n" +
		"  var list = client.value.sub_short_links || []\n" +
		"  for (var i = 0; i < list.length; i++) {\n" +
		"    var l = list[i]\n" +
		"    if (!l || !l.url) continue\n" +
		"    if (formatKey && l.format && l.format !== formatKey) continue\n" +
		"    if (configName && l.config_name && l.config_name !== configName) continue\n" +
		"    if (platform && l.platform && l.platform !== platform) continue\n" +
		"    if (!!providers !== !!l.providers) continue\n" +
		"    return l.url\n" +
		"  }\n" +
		"  return ''\n" +
		"}\n\n" +
		"var buildSubLink = function(formatKey, configName, platform, providers) {\n" +
		"  var short = findShortLink(formatKey, configName, platform, providers)\n" +
		"  if (short) return short\n" +
		"  var base = subUrl.value\n" +
		"  if (!base) return ''\n" +
		"  return base\n" +
		"}\n"

	got := patchPortalJSShortLinks(old)
	if strings.Contains(got, "if (formatKey && l.format && l.format !== formatKey) continue") {
		t.Fatal("宽松匹配没有被升级")
	}
	if !strings.Contains(got, "var wantConfig = configName || ''") {
		t.Fatal("缺少严格匹配")
	}
	if !strings.Contains(got, "var base = longSubUrl.value") {
		t.Fatal("兜底仍然用短链")
	}
}

func TestPatchPortalJSShortLinksLeavesCustomTemplate(t *testing.T) {
	custom := "var foo = 1\n"
	if got := patchPortalJSShortLinks(custom); got != custom {
		t.Fatalf("自定义模板被改写：%q", got)
	}
	if got := patchPortalJSShortLinks(""); got != "" {
		t.Fatal("空 JS 不应改写")
	}
}
