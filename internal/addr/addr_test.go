package addr

import (
	"strings"
	"testing"

	"github.com/MiruSona/officina-asset-tool/internal/meta"
	"github.com/MiruSona/officina-asset-tool/internal/testproj"
)

const (
	gSettings = "5e7a0000000000000000000000000001"
	gDefObj   = "5e7a0000000000000000000000000002"
	gGroup    = "5e7a0000000000000000000000000003"
	gSchemaA  = "5e7a0000000000000000000000000004"
	gSchemaB  = "5e7a0000000000000000000000000005"
	gHero     = "0f1e2d3c4b5a69788796a5b4c3d2e1f0"
	dataDir   = "Assets/AddressableAssetsData/"
)

func scan(t *testing.T, p *testproj.Project) *meta.Table {
	t.Helper()
	tbl, _, err := meta.Scan(p.Root)
	if err != nil {
		t.Fatal(err)
	}
	return tbl
}

// I8 : settings 찾기 세 단계와 기본 자리.
func TestFindSettingsChain(t *testing.T) {
	p := testproj.New(t)
	p.File("ProjectSettings/EditorBuildSettings.asset", testproj.EditorBuildSettings(gDefObj))
	p.Asset("Assets/Config/DefaultObject.asset", gDefObj, testproj.DefaultObject(gSettings))
	p.Asset("Assets/Config/Moved/MySettings.asset", gSettings, testproj.Settings(nil, nil))

	got, notices, err := FindSettings(p.Root, scan(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if got != "Assets/Config/Moved/MySettings.asset" || len(notices) != 0 {
		t.Fatalf("세 단계 = %q %v", got, notices)
	}
}

func TestFindSettingsFallsBack(t *testing.T) {
	cases := map[string]func(p *testproj.Project){
		"EditorBuildSettings 없음": func(p *testproj.Project) {},
		"configObjects 칸 없음": func(p *testproj.Project) {
			p.File("ProjectSettings/EditorBuildSettings.asset", "--- !u!1045 &1\nEditorBuildSettings:\n  m_Scenes: []\n")
		},
		"DefaultObject guid 가 표에 없음": func(p *testproj.Project) {
			p.File("ProjectSettings/EditorBuildSettings.asset", testproj.EditorBuildSettings(gDefObj))
		},
		"settings guid 칸 없음": func(p *testproj.Project) {
			p.File("ProjectSettings/EditorBuildSettings.asset", testproj.EditorBuildSettings(gDefObj))
			p.Asset("Assets/Config/DefaultObject.asset", gDefObj, "--- !u!114 &11400000\nMonoBehaviour:\n  m_Name: DefaultObject\n")
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			p := testproj.New(t)
			setup(p)
			got, notices, err := FindSettings(p.Root, scan(t, p))
			if err != nil {
				t.Fatal(err)
			}
			if got != DefaultSettingsPath {
				t.Fatalf("기본 자리여야 한다 = %q", got)
			}
			if len(notices) != 1 {
				t.Fatalf("알림 한 줄 = %v", notices)
			}
		})
	}
}

func groupProject(t *testing.T, extra string, schemas map[string]string) (*testproj.Project, []string) {
	p := testproj.New(t)
	var order []string
	for _, g := range []string{gSchemaA, gSchemaB} {
		body, ok := schemas[g]
		if !ok {
			continue
		}
		p.Asset(dataDir+"AssetGroups/Schemas/"+g+".asset", g, body)
		order = append(order, g)
	}
	p.Asset(dataDir+"AddressableAssetSettings.asset", gSettings, testproj.Settings([]string{gGroup}, []string{"default"}))
	p.Asset(dataDir+"AssetGroups/G.asset", gGroup, testproj.Group("G", []testproj.Entry{{GUID: gHero, Address: "Hero", Labels: []string{"default"}}}, order, extra))
	return p, order
}

// I5 : includeInBuild 4-4 표 네 줄.
func TestIncludeInBuildRules(t *testing.T) {
	cases := []struct {
		name    string
		extra   string
		schemas map[string]string
		want    bool
		notice  bool
	}{
		{"1 옮겨진 그룹 값", "  m_IncludeInBuild: 0\n  m_IncludeInBuildMigrated: 1\n",
			map[string]string{gSchemaA: testproj.Schema("1", "1")}, false, false},
		{"2 켜진 스키마 값", "  m_IncludeInBuild: 1\n  m_IncludeInBuildMigrated: 0\n",
			map[string]string{gSchemaA: testproj.Schema("0", "1"), gSchemaB: testproj.Schema("1", "0")}, false, false},
		{"3 꺼진 스키마라도 칸이 있으면 그 값", "",
			map[string]string{gSchemaA: testproj.Schema("1", ""), gSchemaB: testproj.Schema("0", "0")}, false, false},
		{"4 아무것도 없으면 true + 알림", "",
			map[string]string{gSchemaA: testproj.Schema("1", "")}, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, _ := groupProject(t, c.extra, c.schemas)
			s, notices, err := Load(p.Root, dataDir+"AddressableAssetSettings.asset", scan(t, p))
			if err != nil {
				t.Fatal(err)
			}
			if len(s.Groups) != 1 || s.Groups[0].IncludeInBuild != c.want {
				t.Fatalf("includeInBuild = %+v", s.Groups)
			}
			if got := len(notices) == 1; got != c.notice {
				t.Fatalf("알림 = %v", notices)
			}
		})
	}
}

func TestLoadGroupsEntriesLabels(t *testing.T) {
	p, _ := groupProject(t, "", map[string]string{gSchemaA: testproj.Schema("1", "1")})
	s, notices, err := Load(p.Root, dataDir+"AddressableAssetSettings.asset", scan(t, p))
	if err != nil || len(notices) != 0 {
		t.Fatal(err, notices)
	}
	if strings.Join(s.Labels, ",") != "default" {
		t.Fatalf("라벨 = %v", s.Labels)
	}
	g := s.Groups[0]
	if g.Name != "G" || len(g.Entries) != 1 || g.Entries[0].Address != "Hero" || g.Entries[0].Labels[0] != "default" {
		t.Fatalf("그룹 = %+v", g)
	}
}

func TestLoadSkipsSpecialAndMissingGroups(t *testing.T) {
	p := testproj.New(t)
	p.Asset(dataDir+"AddressableAssetSettings.asset", gSettings, testproj.Settings([]string{gGroup, "dead0000000000000000000000000000", ""}, nil))
	p.Asset(dataDir+"AssetGroups/Built In Data.asset", gGroup, testproj.Group("Built In Data", []testproj.Entry{
		{GUID: "EditorSceneList", Address: "EditorSceneList"},
		{GUID: "Resources", Address: "Resources"},
	}, nil, ""))
	s, notices, err := Load(p.Root, dataDir+"AddressableAssetSettings.asset", scan(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Groups) != 1 || len(s.Groups[0].Entries) != 0 {
		t.Fatalf("특수 표시는 빠져야 한다 = %+v", s.Groups)
	}
	if s.Labels == nil {
		t.Fatalf("라벨은 빈 목록이어야 한다")
	}
	// 지운 그룹 둘(표에 없는 guid · {fileID: 0}) + 스키마 없는 그룹의 includeInBuild 알림
	if len(notices) != 3 {
		t.Fatalf("알림 = %v", notices)
	}
}

func TestLoadFailures(t *testing.T) {
	cases := map[string]func(p *testproj.Project){
		"settings 없음": func(p *testproj.Project) {},
		"m_GroupAssets 없음": func(p *testproj.Project) {
			p.Asset(dataDir+"AddressableAssetSettings.asset", gSettings, "--- !u!114 &11400000\nMonoBehaviour:\n  m_Name: S\n")
		},
		"m_SerializeEntries 없음": func(p *testproj.Project) {
			p.Asset(dataDir+"AddressableAssetSettings.asset", gSettings, testproj.Settings([]string{gGroup}, nil))
			p.Asset(dataDir+"AssetGroups/G.asset", gGroup, "--- !u!114 &11400000\nMonoBehaviour:\n  m_GroupName: G\n")
		},
		"m_Address 가 모르는 꼴": func(p *testproj.Project) {
			p.Asset(dataDir+"AddressableAssetSettings.asset", gSettings, testproj.Settings([]string{gGroup}, nil))
			p.Asset(dataDir+"AssetGroups/G.asset", gGroup, "--- !u!114 &11400000\nMonoBehaviour:\n  m_GroupName: G\n  m_SerializeEntries:\n  - m_GUID: "+gHero+"\n    m_Address: *ref\n")
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			p := testproj.New(t)
			p.File("Assets/keep.txt", "")
			setup(p)
			if _, _, err := Load(p.Root, dataDir+"AddressableAssetSettings.asset", scan(t, p)); err == nil {
				t.Fatalf("오류여야 한다")
			}
		})
	}
}
