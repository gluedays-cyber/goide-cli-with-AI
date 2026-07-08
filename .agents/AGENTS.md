# 프로젝트 규칙

## 임시 파일 경로

이 앱(`goide`)은 PATH에 등록되어 임의의 디렉터리에서 실행된다. 모든 임시 파일(스크래치 스크립트, 테스트 데이터, `.my_goide` 파일 등)은 앱이 실행된 시점의 현재 작업 디렉터리(CWD, `os.Getwd()` 기준)에 생성해야 한다. 소스 바이너리 위치(`c:\Users\sezzi\programming\goide`), `appDataDir`, 시스템 임시 폴더(`/tmp`, `%TEMP%` 등)에는 생성하지 않는다.
