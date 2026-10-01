# AssetTool

**Unity Addressables 에 올린 에셋(이미지·소리·프리팹)을 읽어 「어떤 주소에 어떤 파일이 있나」 색인 한 장을 만들고, 나중에는 웹에서 보고 고치게 하는 툴이다.**
에셋을 싣고 굽는 일은 Addressables 가 그대로 하고, 이 툴은 그 위에 얹는다. 명령 이름은 `assettool` 이다.

**상태 : 1차 `index` 명령까지 만들었다 (2026-10-01).** 시험 I1~I8 통과, 계약 예제와 바이트가 같다.
다만 **실물 Unity 가 쓴 파일로는 아직 못 돌렸다** — 아래 「한계」.

## 무엇을 하나

게임 데이터 표(DataTool)에서 「이 아이템의 아이콘은 `icon/sword`」처럼 에셋 주소를 적는다고 하자.
그 주소가 정말 있는지, 어떤 그림인지는 Unity 를 열어야 안다. AssetTool 이 그 다리를 놓는다.

- **`assettool index`** — Unity 프로젝트의 Addressables 설정 파일과 `.meta` 를 읽어
  `<Unity 뿌리>/Library/AssetTool/address-index.json` 한 장을 쓴다. Unity 파일은 한 글자도 안 고친다.
- DataTool 은 AssetTool 코드를 부르지 않는다. **이 색인 파일만 읽어서** 주소가 있는지 검사하고, 표 안에서 그림·소리를 보여 준다.

두 툴은 **파일 계약**으로만 잇는다 (아래 「경계 계약」).

## 빌드

```powershell
.\build.ps1          # bin\assettool.exe 를 만든다
.\build.ps1 -Test    # 먼저 go vet · go test 를 돌린다
```

Go 1.26 이상. 바깥 라이브러리 0개, `CGO_ENABLED=0`. `assettool version` 이 빌드한 커밋·시각을 찍는다
(커밋 뒤 손댄 소스로 구웠으면 `-dirty`, git 이 없으면 `dev`).

## 쓰는 법

```
assettool index --unity <Unity 뿌리> [--out <파일>] [--json]
assettool version [--json]
```

- `--out` 기본은 `<Unity 뿌리>/Library/AssetTool/address-index.json`. 다른 자리에 쓰면 `unityRoot` 를 그 자리 기준으로 다시 적는다.
  **다른 드라이브면** 상대경로를 못 만드니 절대 경로를 적고 알림 한 줄 (DataTool 은 그런 색인을 열지 않는다).
- `--out` 이 Unity 뿌리의 `Assets/` · `ProjectSettings/` · `Packages/` 안이면 쓰지 않는다 (종료 1).
- 쓰는 것은 `--out` 한 장뿐이다. 같은 폴더의 tmp 에 쓰고 이름을 바꾸니, 실패해도 옛 색인은 그대로 남는다.
- 알림은 stderr 에 `알림: …` 한 줄씩. `--json` 이면 stdout 에 한 줄로 낸다.
  - 성공 : `{"ok":true,"out":"…","counts":{"entries":3,"groups":3},"notices":[…]}`
  - 실패 : `{"ok":false,"exit":4,"error":"…","notices":[…]}`

| 종료 | 뜻 |
| --- | --- |
| 0 | 성공 (알림이 있어도) |
| 1 | 사용법 잘못 · `--out` 이 Unity 폴더 안 |
| 4 | 읽기 실패 · 읽어야 할 칸이 모르는 꼴. **색인을 안 쓴다** — 반쪽 색인은 거짓 통과를 부른다 |
| 5 | 쓰기 실패 |

알림이 나는 경우(지운 그룹 · 경로를 못 푼 항목 · 같은 address 둘 · guid 가 겹친 `.meta` 등)와
읽는 차례는 `Docs/Design/2026-09-23-index설계.md` 4-2 · 4-5 를 본다.

## 경계 계약 (계약 버전 1)

> 스튜디오 저장소 루트 Docs 의 2026-09-23 「AssetTool 과 DataTool 연동 설계」 2절을 그대로 옮긴 글이다. DataTool README 에도 같은 글이 있다.
> 고칠 때는 두 README 와 두 `Testdata/contract/address-index.example.json` 을 같은 날 고친다.

### 2-1. 색인 파일 `address-index.json`

AssetTool `index` 가 **쓰고**, DataTool·AssetTool 웹이 **읽는다.** UTF-8, BOM 없음. 쓸 때는 tmp → 이름 바꾸기라 반쯤 쓴 파일을 읽는 일이 없다.

**맨 위 칸**

| 칸 | 타입 | 필수 | 뜻 |
| --- | --- | --- | --- |
| `version` | int | ✓ | 계약 버전. 지금 `1` |
| `generator` | string | ✓ | 쓴 툴과 툴 버전 (`"assettool 0.1.0 (a1b2c3d)"`). 사람이 보는 값, 기계는 안 본다 |
| `generatedAt` | string | ✓ | 만든 시각, UTC, Go `time.RFC3339Nano` 꼴 (`2026-09-23T05:12:00Z`) |
| `unityRoot` | string | ✓ | 색인 파일이 있는 폴더 기준 Unity 뿌리 (`"../.."`). 다른 드라이브면 절대 경로 |
| `settingsPath` | string | ✓ | 읽은 `AddressableAssetSettings.asset` 자리 (뿌리 기준) |
| `sourceMtime` | string | ✓ | 읽은 설정 폴더(`settingsPath` 의 폴더) 안 `*.asset` 중 **가장 새 mtime**, UTC, `time.RFC3339Nano` |
| `labels` | string[] | ✓ | 라벨 전체 목록 (`m_LabelTable.m_LabelNames`). 없으면 `[]` |
| `entries` | object[] | ✓ | 항목. 차례는 아래 「쓰기 꼴」 |

**`entries[]` 한 칸**

| 칸 | 타입 | 필수 | 뜻 |
| --- | --- | --- | --- |
| `address` | string | ✓ | Addressables address 그대로 |
| `guid` | string | ✓ | 에셋 guid (32자 16진) |
| `path` | string | ✓ | 뿌리 기준 상대경로, `/` 구분, `Assets/` 또는 `Packages/` 로 시작. **빈 값 = 경로를 못 풀었다** |
| `kind` | string | ✓ | `image` · `audio` · `prefab` · `scene` · `other` 다섯 중 하나 (아래 표). 경로를 못 풀었으면 `other` |
| `group` | string | ✓ | 그룹 이름 (`m_GroupName`) |
| `includeInBuild` | bool | ✓ | 빌드에 들어가나 (4-4 규칙) |
| `labels` | string[] | ✓ | 이 항목의 라벨. 없으면 `[]` |
| `fromFolder` | string | — | 폴더 항목을 펼친 것이면 그 폴더의 address |
| `sub` | object[] | — | 하위 에셋. 지금은 **스프라이트 시트만** : `{"name": string, "rect": {"x","y","w","h": number}}`. `rect` 는 픽셀, **y 는 아래에서 잰다**(Unity 꼴 그대로). **`sub` 가 없으면 「하위를 모른다」** 이지 「하위가 없다」가 아니다 (FBX·spriteatlas 하위도 `address[이름]` 으로 부른다) |

**`kind` 판정 (확장자, 소문자로 견준다)**

| kind | 확장자 |
| --- | --- |
| `image` | png jpg jpeg gif bmp tga psd psb tif tiff exr hdr |
| `audio` | wav mp3 ogg aif aiff flac xm mod it s3m |
| `prefab` | prefab |
| `scene` | unity |
| `other` | 나머지 전부 (`.asset` · `.mat` · `.spriteatlas` · `.fbx` …) |

**쓰기 꼴 (같은 입력이면 같은 바이트)**

- 차례 : `entries` 는 **address 바이트 차례**(Go `sort.Strings` 와 같다 — 대문자가 소문자 앞), address 가 같으면 guid 차례.
- 꼴 : Go `json.MarshalIndent(v, "", "  ")` 과 같은 들여쓰기(2칸) · HTML 이스케이프 안 함(`SetEscapeHTML(false)`) · 끝에 `\n` 하나 · 줄끝 LF. 칸 차례는 위 표 차례.
- 시각 : `time.RFC3339Nano` (UTC, 뒤쪽 0 은 Go 가 떼는 대로).

**그 밖의 규칙**

- 같은 address 가 두 항목에 있을 수 있다 (Addressables 가 막지 않는다). 색인은 둘 다 넣고, `index` 는 알림 한 줄을 낸다.
- **계약 버전 규칙 :** 같은 `version` 안에서는 **칸을 더하기만** 한다. 읽는 쪽은 모르는 칸을 무시한다. 칸을 빼거나 뜻을 바꾸면 `version` 을 올린다.
- **모르는 `version`** 이면 읽는 쪽은 추측하지 않고 멈춘다 — 「색인 계약 버전 N 을 모른다. DataTool 을 새로 빌드하거나 AssetTool 버전을 맞춰라」.

### 2-2. 예제 한 벌

```json
{
  "version": 1,
  "generator": "assettool 0.1.0 (a1b2c3d)",
  "generatedAt": "2026-09-23T05:12:00Z",
  "unityRoot": "../..",
  "settingsPath": "Assets/AddressableAssetsData/AddressableAssetSettings.asset",
  "sourceMtime": "2026-09-23T05:10:41.123456789Z",
  "labels": [
    "default",
    "ui"
  ],
  "entries": [
    {
      "address": "Hero",
      "guid": "0f1e2d3c4b5a69788796a5b4c3d2e1f0",
      "path": "Assets/Prefabs/Hero.prefab",
      "kind": "prefab",
      "group": "Default Local Group",
      "includeInBuild": true,
      "labels": [
        "default"
      ]
    },
    {
      "address": "Sfx/hit.wav",
      "guid": "1234567890abcdef1234567890abcdef",
      "path": "Assets/Audio/Sfx/hit.wav",
      "kind": "audio",
      "group": "Audio",
      "includeInBuild": true,
      "labels": [],
      "fromFolder": "Sfx"
    },
    {
      "address": "icons",
      "guid": "9a8b7c6d5e4f30211203f4e5d6c7b8a9",
      "path": "Assets/Art/icons.png",
      "kind": "image",
      "group": "UI",
      "includeInBuild": true,
      "labels": [
        "ui"
      ],
      "sub": [
        {
          "name": "icon_sword",
          "rect": {
            "x": 0,
            "y": 64,
            "w": 64,
            "h": 64
          }
        },
        {
          "name": "icon_potion",
          "rect": {
            "x": 64,
            "y": 64,
            "w": 64,
            "h": 64
          }
        }
      ]
    }
  ]
}
```

**이 파일은 `Testdata/contract/address-index.example.json` 으로 두 툴에 같은 바이트로 둔다.**
두 저장소 `.gitattributes` 에 `Testdata/contract/*.json -text` 를 더한다 — git 이 줄끝을 CRLF 로 바꾸면 바이트 대조가 깨진다.
DataTool 은 이것을 읽어 시험하고, AssetTool 은 시험 자료 프로젝트를 색인해 **이것과 바이트가 같은지** 본다(`generatedAt`·`generator`·`sourceMtime` 은 시험이 고정값으로 넣는다).
계약을 고치면 두 파일을 같은 날 고치고, 양쪽 README 의 이 절도 같이 고친다.

### 2-3. 썸네일 폴더 (2차부터 쓴다, 자리는 지금 정한다)

| 규칙 | 값 |
| --- | --- |
| 자리 | `<Unity 뿌리>/Library/AssetTool/thumbs/<guid>.png` — **색인 자리와 상관없이 늘 여기** |
| 파일 이름 | 에셋 guid 소문자 32자 + `.png`. address 가 아니라 guid 인 까닭 : address 는 바뀌어도 guid 는 안 바뀐다 |
| 크기 · 꼴 | **128×128 정사각, RGBA PNG, 투명 배경.** 물체는 경계 상자를 가운데 맞춰 채운다 |
| 누가 | AssetTool Unity 쪽이 쓴다 / DataTool·AssetTool 웹은 **있으면 보이고 없으면 「썸네일 없음」** |

### 2-4. 칸 값 규칙

`asset` 칸 값 = address 그대로. 하위 에셋은 `address[이름]` (Addressables 하위 객체 문법). **빈 문자열은 「없음」** 이다. 라벨은 안 받는다.

## 한계

- **실물 Unity 가 쓴 파일로는 못 봤다.** `Testdata/project/` 는 Unity YAML 꼴을 알고 손으로 쓴 흉내다
  (공식 Addressables-Sample 은 라이선스 확인 전이라 안 가져왔다). 긴 값을 접는 꼴 · 한글 address 를 적는 꼴은 실물로 확인해야 한다.
- **따옴표 없는 값 안에 ` #` 이 있으면 그 칸을 못 읽는다(종료 4).** 주석으로 잘라 조용히 틀리는 것보다 낫다고 봤다. Unity 가 이런 address 를 따옴표로 감싸 쓰는지 실물로 보고 풀 수 있다.
- **`.psb` (PSD Importer) 는 `sub` 를 안 만든다.** `.meta` 꼴이 달라서다. 칸에 `[이름]` 을 적으면 DataTool 이 경고로 통과시킨다.
- 스프라이트 시트(Multiple)인데 자른 칸이 0개면 `sub` 를 안 적는다 — 「하위를 모른다」로 읽힌다.
- `Packages/` 안 에셋은 `.meta` 를 안 훑으니 `path:""` · `kind:"other"` 로 들어가고 알림이 난다.
- `sourceMtime` 은 settings 폴더의 `*.asset` 만 본다. 폴더 항목 안에 파일만 새로 넣거나 스프라이트 시트를 다시 자르면(`.meta` 만 바뀜) 낡음이 안 잡힌다.

## 폴더

```
cmd/assettool/        인자 가르기 · 결과 찍기 (index · version)
internal/unityyaml/   Unity YAML 줄 읽기. Addressables 를 모른다
internal/meta/        .meta 훑기 → guid 표 · 스프라이트 시트
internal/addr/        settings·그룹·스키마 읽기. Addressables 칸 이름을 아는 유일한 곳
internal/index/       계약 꼴 · kind 판정 · 폴더 펼치기 · 정렬 · 쓰기. 계약을 아는 유일한 곳
internal/testproj/    시험이 임시 Unity 뿌리를 만드는 도구 (시험에서만 쓴다)
Testdata/contract/    address-index.example.json (DataTool 과 같은 바이트)
Testdata/project/     작은 Unity 뿌리 흉내. 색인하면 위 예제가 나온다
```

## 차례

| 차수 | AssetTool 이 하는 것 |
| --- | --- |
| **1차** | `index` 명령 하나 — **만듦** |
| 2차 | 웹 보기(`serve`) · 규칙 파일 편집 · Unity 쪽 규칙 적용 · 프리팹 썸네일 굽기 |
| 3차 | Unity 쪽 암호화 · 경로 훅 |

`index` 설계는 `Docs/Design/2026-09-23-index설계.md`, 할 일은 `Docs/Todo/할일.md` 를 본다.

## 라이선스

MIT. `LICENSE` 를 본다.
