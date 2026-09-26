# AssetTool

**Unity Addressables 에 올린 에셋(이미지·소리·프리팹)을 읽어 「어떤 주소에 어떤 파일이 있나」 색인 한 장을 만들고, 나중에는 웹에서 보고 고치게 하는 툴이다.**
에셋을 싣고 굽는 일은 Addressables 가 그대로 하고, 이 툴은 그 위에 얹는다. 명령 이름은 `assettool` 이다.

**상태 : 아직 구현 전 (2026-09-26).** 설계만 끝났다. 설계는 스튜디오 저장소 루트 Docs 의 2026-09-23 「AssetTool 과 DataTool 연동 설계」에 있다.
두 툴에 걸친 문서라 이 저장소가 아니라 스튜디오 쪽에 둔다.

## 무엇을 하나

게임 데이터 표(DataTool)에서 「이 아이템의 아이콘은 `icon/sword`」처럼 에셋 주소를 적는다고 하자.
그 주소가 정말 있는지, 어떤 그림인지는 Unity 를 열어야 안다. AssetTool 이 그 다리를 놓는다.

- **`assettool index`** — Unity 프로젝트의 Addressables 설정 파일과 `.meta` 를 읽어
  `<Unity 뿌리>/Library/AssetTool/address-index.json` 한 장을 쓴다. Unity 파일은 한 글자도 안 고친다.
- DataTool 은 AssetTool 코드를 부르지 않는다. **이 색인 파일만 읽어서** 주소가 있는지 검사하고, 표 안에서 그림·소리를 보여 준다.

두 툴은 **파일 계약**으로만 잇는다.

| 파일 | 쓰는 쪽 | 읽는 쪽 |
| --- | --- | --- |
| `Library/AssetTool/address-index.json` | AssetTool `index` | DataTool · AssetTool 웹 |
| `Library/AssetTool/thumbs/<guid>.png` (2차부터) | AssetTool 썸네일 굽기 | DataTool · AssetTool 웹 |

## 차례

| 차수 | AssetTool 이 하는 것 |
| --- | --- |
| **1차** | `index` 명령 하나 (Go 단일 exe, 바깥 라이브러리 0개) |
| 2차 | 웹 보기(`serve`) · 규칙 파일 편집 · Unity 쪽 규칙 적용 · 프리팹 썸네일 굽기 |
| 3차 | Unity 쪽 암호화 · 경로 훅 |

할 일은 `Docs/Todo/할일.md` 를 본다.

## 라이선스

MIT. `LICENSE` 를 본다.
