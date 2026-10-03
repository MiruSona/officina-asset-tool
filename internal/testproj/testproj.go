// Package testproj 는 시험이 임시 폴더에 작은 Unity 뿌리 흉내를 만드는 도구다. 시험에서만 쓴다.
package testproj

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Project 는 임시 Unity 뿌리 하나다.
type Project struct {
	t    testing.TB
	Root string
}

func New(t testing.TB) *Project {
	t.Helper()
	return &Project{t: t, Root: t.TempDir()}
}

// File 은 뿌리 기준 rel 에 글을 쓴다.
func (p *Project) File(rel, body string) {
	p.t.Helper()
	full := filepath.Join(p.Root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		p.t.Fatal(err)
	}
}

// Asset 은 파일과 그 .meta 를 같이 쓴다.
func (p *Project) Asset(rel, guid, body string) {
	p.t.Helper()
	p.File(rel, body)
	p.File(rel+".meta", Meta(guid))
}

// Folder 는 폴더와 그 .meta 를 만든다.
func (p *Project) Folder(rel, guid string) {
	p.t.Helper()
	full := filepath.Join(p.Root, filepath.FromSlash(rel))
	if err := os.MkdirAll(full, 0o755); err != nil {
		p.t.Fatal(err)
	}
	p.File(rel+".meta", "fileFormatVersion: 2\nguid: "+guid+"\nfolderAsset: yes\nDefaultImporter:\n  externalObjects: {}\n  userData: \n")
}

// CopyDir 는 src 폴더를 뿌리 기준 rel 아래로 통째로 복사한다.
func (p *Project) CopyDir(src, rel string) {
	p.t.Helper()
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		sub, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		p.File(rel+"/"+filepath.ToSlash(sub), string(data))
		return nil
	})
	if err != nil {
		p.t.Fatal(err)
	}
}

func Meta(guid string) string {
	return "fileFormatVersion: 2\nguid: " + guid + "\nNativeFormatImporter:\n  externalObjects: {}\n  mainObjectFileID: 11400000\n  userData: \n"
}

const header = "%YAML 1.1\n%TAG !u! tag:unity3d.com,2011:\n--- !u!114 &11400000\nMonoBehaviour:\n  m_ObjectHideFlags: 0\n  m_Enabled: %s\n  m_Script: {fileID: 11500000, guid: 468a46d0ae32c3544b7d98094e6448a9, type: 3}\n"

func ref(guid string) string {
	if guid == "" {
		return "{fileID: 0}"
	}
	return "{fileID: 11400000, guid: " + guid + ", type: 2}"
}

func list(indent string, items []string) string {
	if len(items) == 0 {
		return " []\n"
	}
	var b strings.Builder
	b.WriteString("\n")
	for _, it := range items {
		b.WriteString(indent + "- " + it + "\n")
	}
	return b.String()
}

// Settings 는 AddressableAssetSettings.asset 글이다.
func Settings(groupGUIDs []string, labels []string) string {
	refs := make([]string, len(groupGUIDs))
	for i, g := range groupGUIDs {
		refs[i] = ref(g)
	}
	return strings.Replace(header, "%s", "1", 1) +
		"  m_Name: AddressableAssetSettings\n" +
		"  m_GroupAssets:" + list("  ", refs) +
		"  m_LabelTable:\n    m_LabelNames:" + list("    ", labels)
}

// Entry 는 그룹 안 항목 하나다.
type Entry struct {
	GUID    string
	Address string
	Labels  []string
}

// Group 은 그룹 .asset 글이다. extra 는 m_IncludeInBuild 같은 칸을 그대로 넣는다.
func Group(name string, entries []Entry, schemaGUIDs []string, extra string) string {
	var b strings.Builder
	b.WriteString(strings.Replace(header, "%s", "1", 1))
	b.WriteString("  m_Name: " + name + "\n  m_GroupName: " + name + "\n")
	b.WriteString("  m_SerializeEntries:")
	if len(entries) == 0 {
		b.WriteString(" []\n")
	} else {
		b.WriteString("\n")
	}
	for _, e := range entries {
		fmt.Fprintf(&b, "  - m_GUID: %s\n    m_Address: %s\n    m_ReadOnly: 0\n    m_SerializedLabels:%s", e.GUID, e.Address, list("    ", e.Labels))
		b.WriteString("    FlaggedDuringContentUpdateRestriction: 0\n")
	}
	b.WriteString("  m_ReadOnly: 0\n")
	b.WriteString(extra)
	refs := make([]string, len(schemaGUIDs))
	for i, g := range schemaGUIDs {
		refs[i] = ref(g)
	}
	b.WriteString("  m_SchemaSet:\n    m_Schemas:" + list("    ", refs))
	return b.String()
}

// Schema 는 그룹 스키마 .asset 글이다. include 가 "" 이면 m_IncludeInBuild 칸이 없는 스키마다.
func Schema(enabled, include string) string {
	s := strings.Replace(header, "%s", enabled, 1) + "  m_Name: Schema\n  m_Compression: 1\n"
	if include != "" {
		s += "  m_IncludeInBuild: " + include + "\n"
	}
	return s + "  m_BundleMode: 0\n"
}

// DefaultObject 는 AddressableAssetSettingsDefaultObject 글이다.
func DefaultObject(settingsGUID string) string {
	return strings.Replace(header, "%s", "1", 1) + "  m_Name: DefaultObject\n  m_AddressableAssetSettingsGuid: " + settingsGUID + "\n"
}

// EditorBuildSettings 는 ProjectSettings/EditorBuildSettings.asset 글이다.
func EditorBuildSettings(defaultObjectGUID string) string {
	return "%YAML 1.1\n%TAG !u! tag:unity3d.com,2011:\n--- !u!1045 &1\nEditorBuildSettings:\n  m_ObjectHideFlags: 0\n  serializedVersion: 2\n  m_Scenes: []\n" +
		"  m_configObjects:\n    com.unity.addressableassets: " + ref(defaultObjectGUID) + "\n"
}
