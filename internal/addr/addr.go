// Package addr 는 Addressables 설정·그룹·스키마 .asset 을 읽는다. Addressables 칸 이름을 아는 유일한 곳이다.
// 읽는 차례와 규칙은 Docs/Design/2026-09-23-index설계.md 4-2·4-4·4-5.
package addr

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MiruSona/officina-asset-tool/internal/meta"
	"github.com/MiruSona/officina-asset-tool/internal/unityyaml"
)

// DefaultSettingsPath 는 찾기 세 단계 중 어느 칸이든 없을 때 쓰는 기본 자리다.
const DefaultSettingsPath = "Assets/AddressableAssetsData/AddressableAssetSettings.asset"

const editorBuildSettingsPath = "ProjectSettings/EditorBuildSettings.asset"

// Entry 는 그룹 안 항목 하나다 (폴더 펼치기 전).
type Entry struct {
	GUID    string
	Address string
	Labels  []string
}

// Group 은 읽은 그룹 하나다.
type Group struct {
	Name           string
	Path           string
	IncludeInBuild bool
	Entries        []Entry
}

// Settings 는 settings .asset 과 그 그룹들이다.
type Settings struct {
	Path   string
	Labels []string
	Groups []Group
}

// 옛 Built In Data 그룹이 guid 자리에 쓰는 특수 표시다.
var specialGUIDs = map[string]bool{"EditorSceneList": true, "Resources": true}

// readBody 는 rel 파일의 첫 MonoBehaviour(또는 className) 몸통을 준다.
func readBody(root, rel, className string) (*unityyaml.Node, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	docs, err := unityyaml.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	for _, d := range docs {
		body, err := d.Root.Lookup(className)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		if body != nil {
			return body, nil
		}
	}
	return nil, fmt.Errorf("%s: %s 문서가 없다", rel, className)
}

// str 은 칸 하나를 글자로 읽는다. 칸이 없으면 ok=false.
func str(n *unityyaml.Node, path ...string) (string, bool, error) {
	v, err := n.Lookup(path...)
	if err != nil || v == nil {
		return "", false, err
	}
	s, err := v.String()
	if err != nil {
		return "", false, err
	}
	return s, true, nil
}

// FindSettings 는 EditorBuildSettings → DefaultObject → settings 세 단계로 settings 자리를 찾는다.
// 파일·칸이 없으면 기본 자리와 알림 한 줄, 있는데 모르는 꼴이면 오류다.
func FindSettings(root string, t *meta.Table) (string, []string, error) {
	fallback := func(why string) (string, []string, error) {
		return DefaultSettingsPath, []string{"settings 자리를 못 찾아 기본 자리를 쓴다 (" + why + ")"}, nil
	}

	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(editorBuildSettingsPath))); errors.Is(err, os.ErrNotExist) {
		return fallback(editorBuildSettingsPath + " 없음")
	}
	ebs, err := readBody(root, editorBuildSettingsPath, "EditorBuildSettings")
	if err != nil {
		return "", nil, err
	}
	defGUID, ok, err := str(ebs, "m_configObjects", "com.unity.addressableassets", "guid")
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", editorBuildSettingsPath, err)
	}
	if !ok {
		return fallback("m_configObjects 에 com.unity.addressableassets 없음")
	}
	defObj, found := t.ByGUID(defGUID)
	if !found {
		return fallback("DefaultObject guid " + defGUID + " 가 .meta 표에 없음")
	}

	body, err := readBody(root, defObj.Path, "MonoBehaviour")
	if err != nil {
		return "", nil, err
	}
	setGUID, ok, err := str(body, "m_AddressableAssetSettingsGuid")
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", defObj.Path, err)
	}
	if !ok {
		return fallback(defObj.Path + " 에 m_AddressableAssetSettingsGuid 없음")
	}
	settings, found := t.ByGUID(setGUID)
	if !found {
		return fallback("settings guid " + setGUID + " 가 .meta 표에 없음")
	}
	return settings.Path, nil, nil
}

// refGUID 는 `{fileID: …, guid: …, type: 2}` 의 guid 를 준다. `{fileID: 0}` 이면 빈 값.
func refGUID(n *unityyaml.Node) (string, error) {
	g, _, err := str(n, "guid")
	return g, err
}

// Load 는 settings 와 그 그룹·스키마를 읽는다. 오류면 색인을 쓰지 않는다 (반쪽 색인은 거짓 통과를 부른다).
func Load(root, settingsPath string, t *meta.Table) (*Settings, []string, error) {
	var notices []string
	body, err := readBody(root, settingsPath, "MonoBehaviour")
	if err != nil {
		return nil, nil, fmt.Errorf("settings 를 못 읽었다: %w", err)
	}
	wrap := func(err error) error { return fmt.Errorf("%s: %w", settingsPath, err) }

	s := &Settings{Path: settingsPath, Labels: []string{}}
	labels, err := body.Lookup("m_LabelTable", "m_LabelNames")
	if err != nil {
		return nil, nil, wrap(err)
	}
	if labels != nil {
		if s.Labels, err = stringList(labels); err != nil {
			return nil, nil, wrap(err)
		}
	}

	groupList, err := body.Lookup("m_GroupAssets")
	if err != nil {
		return nil, nil, wrap(err)
	}
	if groupList == nil {
		return nil, nil, fmt.Errorf("%s: m_GroupAssets 칸이 없다", settingsPath)
	}
	refs, err := groupList.List()
	if err != nil {
		return nil, nil, wrap(err)
	}
	for _, r := range refs {
		guid, err := refGUID(r)
		if err != nil {
			return nil, nil, wrap(err)
		}
		a, ok := t.ByGUID(guid)
		if guid == "" || !ok {
			notices = append(notices, fmt.Sprintf("그룹 guid %q 가 .meta 표에 없어 건너뛴다 (지운 그룹)", guid))
			continue
		}
		g, gNotices, err := loadGroup(root, a.Path, t)
		notices = append(notices, gNotices...)
		if err != nil {
			return nil, notices, err
		}
		s.Groups = append(s.Groups, *g)
	}
	return s, notices, nil
}

func stringList(n *unityyaml.Node) ([]string, error) {
	items, err := n.List()
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, it := range items {
		v, err := it.String()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func loadGroup(root, rel string, t *meta.Table) (*Group, []string, error) {
	body, err := readBody(root, rel, "MonoBehaviour")
	if err != nil {
		return nil, nil, fmt.Errorf("그룹을 못 읽었다: %w", err)
	}
	wrap := func(err error) error { return fmt.Errorf("%s: %w", rel, err) }

	name, ok, err := str(body, "m_GroupName")
	if err != nil {
		return nil, nil, wrap(err)
	}
	if !ok {
		return nil, nil, fmt.Errorf("%s: m_GroupName 칸이 없다", rel)
	}
	g := &Group{Name: name, Path: rel}
	var notices []string

	list, err := body.Lookup("m_SerializeEntries")
	if err != nil {
		return nil, nil, wrap(err)
	}
	if list == nil {
		return nil, nil, fmt.Errorf("%s: m_SerializeEntries 칸이 없다", rel)
	}
	items, err := list.List()
	if err != nil {
		return nil, nil, wrap(err)
	}
	for _, it := range items {
		e, skip, noAddress, err := readEntry(it)
		if err != nil {
			return nil, notices, wrap(err)
		}
		if noAddress {
			notices = append(notices, fmt.Sprintf("%s : 줄 %d 항목 guid %s 에 m_Address 칸이 없어 빈 address 로 넣는다", rel, it.Line, e.GUID))
		}
		if !skip {
			g.Entries = append(g.Entries, e)
		}
	}

	include, notice, err := includeInBuild(root, body, t)
	if err != nil {
		return nil, notices, wrap(err)
	}
	g.IncludeInBuild = include
	if notice != "" {
		notices = append(notices, rel+" : "+notice)
	}
	return g, notices, nil
}

// readEntry 는 항목 하나를 읽는다. skip 은 특수 표시, noAddress 는 m_Address 칸이 아예 없다는 뜻이다.
func readEntry(it *unityyaml.Node) (Entry, bool, bool, error) {
	var e Entry
	guid, ok, err := str(it, "m_GUID")
	if err != nil {
		return e, false, false, err
	}
	if !ok {
		return e, false, false, &unityyaml.Error{Line: it.Line, Msg: "항목에 m_GUID 칸이 없다"}
	}
	if specialGUIDs[guid] {
		return e, true, false, nil
	}
	e.GUID = guid
	address, hasAddress, err := str(it, "m_Address")
	if err != nil {
		return e, false, false, err
	}
	e.Address = address
	e.Labels = []string{}
	labels, err := it.Lookup("m_SerializedLabels")
	if err != nil {
		return e, false, false, err
	}
	if labels != nil {
		if e.Labels, err = stringList(labels); err != nil {
			return e, false, false, err
		}
	}
	return e, false, !hasAddress, nil
}

type schemaInfo struct {
	hasInclude bool
	include    bool
	enabled    bool
}

// includeInBuild 는 4-4 표를 위에서부터 처음 맞는 줄로 정한다.
func includeInBuild(root string, group *unityyaml.Node, t *meta.Table) (bool, string, error) {
	migrated, ok, err := flag(group, "m_IncludeInBuildMigrated")
	if err != nil {
		return false, "", err
	}
	if ok && migrated {
		v, has, err := flag(group, "m_IncludeInBuild")
		if err != nil {
			return false, "", err
		}
		if has {
			return v, "", nil
		}
	}

	schemas, notices, err := readSchemas(root, group, t)
	if err != nil {
		return false, "", err
	}
	for _, s := range schemas {
		if s.hasInclude && s.enabled {
			return s.include, notices, nil
		}
	}
	for _, s := range schemas {
		if s.hasInclude {
			return s.include, notices, nil
		}
	}
	msg := "빌드 포함 여부를 정하는 칸이 없어 true 로 둔다"
	if notices != "" {
		msg = notices + " · " + msg
	}
	return true, msg, nil
}

func flag(n *unityyaml.Node, key string) (bool, bool, error) {
	v, err := n.Lookup(key)
	if err != nil || v == nil {
		return false, false, err
	}
	b, err := v.Flag()
	return b, err == nil, err
}

func readSchemas(root string, group *unityyaml.Node, t *meta.Table) ([]schemaInfo, string, error) {
	list, err := group.Lookup("m_SchemaSet", "m_Schemas")
	if err != nil || list == nil {
		return nil, "", err
	}
	refs, err := list.List()
	if err != nil {
		return nil, "", err
	}
	var out []schemaInfo
	var missing []string
	for _, r := range refs {
		guid, err := refGUID(r)
		if err != nil {
			return nil, "", err
		}
		a, ok := t.ByGUID(guid)
		if guid == "" || !ok {
			missing = append(missing, fmt.Sprintf("스키마 guid %q 가 .meta 표에 없어 건너뛴다", guid))
			continue
		}
		body, err := readBody(root, a.Path, "MonoBehaviour")
		if err != nil {
			return nil, "", err
		}
		var s schemaInfo
		if s.include, s.hasInclude, err = flag(body, "m_IncludeInBuild"); err != nil {
			return nil, "", fmt.Errorf("%s: %w", a.Path, err)
		}
		if s.enabled, _, err = flag(body, "m_Enabled"); err != nil {
			return nil, "", fmt.Errorf("%s: %w", a.Path, err)
		}
		out = append(out, s)
	}
	return out, strings.Join(missing, " · "), nil
}
