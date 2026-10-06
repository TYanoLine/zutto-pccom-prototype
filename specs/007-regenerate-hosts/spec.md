# Spec: 生成ホストの再生成（1r）と電話帳の堅牢化

**Spec ID**: 007-regenerate-hosts  
**Status**: Draft  
**Depends on**: spec 006（マージ済み）

## 背景

spec 006（改訂 2）により、電話帳（センターディレクトリ）はプリセット局（`GET /api/directory`）を先頭に、ブラウザごとの世界に紐づく生成局（`/api/world/bootstrap`）を後ろに並べる構成となった。

しかし、現在の挙動には以下の課題が存在する。

1. **ホスト定義を作り直す手段がない**:
   生成された 100 局のバリエーションを変えたい場合、現在は世界まるごと削除する `ResetWorld` や 1 局ごとの `ResetHost` が Go の内部コードにあるのみで、HTTP のルートがなく、ブラウザや端末から実行できない。世界の鍵や電話番号の割り当て（シードに基づく番号体系）を維持したまま、局名・スペック・世代番号だけを更新する手段が必要である。
2. **生成処理が HTTP リクエストに依存している**:
   `/api/world/bootstrap` の初回生成時、クライアントが切断（タイムアウト、タブを閉じる、再読み込みなど）すると、`r.Context()` のキャンセルによって LLM（Azure OpenAI）の呼び出しや DB 保存が中断されてしまう。これにより、LLM API の呼び出しが無駄になり、次回アクセス時に再び最初から生成し直すことになってしまう。
3. **生成局の取得失敗時に再試行されず、状況が分からない**:
   Render の起動遅延等で生成局の取得が失敗した場合、現在は再試行が行われず、電話帳画面の注記（「ほかのセンターを読み込み中...」）が消えるだけで、失敗したことが分からない。
4. **プリセット局取得失敗時に画面が止まる**:
   起動直後のプリセット局取得中にメインメニューで `1` を押すと「センター情報の読み込み」画面が表示されるが、その後 API 取得が失敗した際にエラー画面（`showDirectoryError`）に描き直されず、読み込み中画面のまま止まる。

## ユーザーストーリー

### US1: メインメニューで `1r` と入力すると、生成ホストだけが作り直される（Priority: P1）

運用者・利用者として、メインメニューで `1r` と入力することで、現在の世界のシードと電話番号を保ったまま、生成ホストの名前とスペックだけを LLM で再生成したい。

**Independent Test**:
1. `POST /api/world/reset-hosts?key=<世界の鍵>` を呼び出すと、世界の行・鍵・シード・各局の電話番号は変わらず、局名・スペックが更新され、各局の `generation` が上がる。
2. Web のメインメニューで `1r` を入力すると、端末が再生成中画面になり、完了後にセンター・リスト（プリセット局 + 新しい生成局の一覧）が自動で開く。

### US2: クライアントが切断しても、生成と保存が完走する（Priority: P1）

運用者として、LLM の局名生成中にクライアントがタイムアウトまたは切断しても、サーバ側の生成と DB 保存が最後まで完了してほしい。

**Independent Test**:
クライアントがリクエストを送信して直後に切断しても、サーバのログで生成と DB コミットが正常に完了し、その後のリクエストで生成済みの局が即座に返る。

### US3: 生成局の取得失敗時に自動再試行される（Priority: P2）

利用者として、生成局の取得が一時的なエラーや遅延で失敗しても、バックグラウンドで自動的に再試行されて届いてほしい。

**Independent Test**:
生成局の取得が失敗した場合、2秒、10秒、30秒の間隔、およびブラウザの `online` イベントで自動的に再取得が行われ、成功時に電話帳一覧が更新される。

### US4: 生成局の読み込み失敗が電話帳に表示される（Priority: P2）

利用者として、生成局の読み込みが失敗している間、電話帳の下部にその旨が表示されてほしい。

**Independent Test**:
生成局の取得失敗中、電話帳画面（`TerminalCenterDirectory`）の下部に「ほかのセンターの読み込みに失敗しました（再試行中...）」が表示され、再取得に成功するとクリアされる。

### US5: プリセット局の読み込み失敗時にエラー画面に切り替わる（Priority: P1）

利用者として、プリセット局の読み込み待ち中にエラーが発生した場合、エラー画面に切り替わって ESC でメインメニューに戻れるようにしてほしい。

**Independent Test**:
`openDirectoryWhenReadyRef` が true（`1` を押して読み込み待ち）の状態でプリセット局の取得が失敗したとき、端末画面が直ちに `showDirectoryError()` のエラー画面に切り替わる。

## 要件

### サーバ要件

- **FR-001（全局リセット API）**:
  サーバは `POST /api/world/reset-hosts?key=<世界の鍵>` を提供する。
  - メソッド: `POST` のみ（それ以外は 405 Method Not Allowed、`Allow: POST`）。
  - クエリパラメータ: `key`（32桁の16進数 hex）。不正なキーは 400 Bad Request（`{"error":"invalid world key"}`）。
  - DB が未設定の場合は 503 Service Unavailable。
  - 世界が存在しない場合は 404 Not Found（`{"error":"world not found"}`）。
  - 認可: 世界の鍵のみ（トークン不要）。※将来、世界にプレイヤーの状態が保存されるようになった段階でトークン必須に見直す。
  - 応答: 200 OK、JSON 形式:
    ```json
    {
      "worldId": "<uuid>",
      "centers": [ ... ],
      "source": "azure_openai",
      "model": "<model名>"
    }
    ```
- **FR-002（`worldcatalog.Store.ResetHosts`）**:
  - `Store` に `ResetHosts(ctx context.Context, worldKey string, count int, generateNames func(context.Context, int) ([]string, error)) (Catalog, error)` を追加する。
  - 世界の鍵でアドバイザリロック（`lockWorld`）を取得する。
  - 対象世界の `seed` と現在の最大 `generation`（`SELECT COALESCE(MAX(generation), 0) FROM hosts WHERE world_id=$1::uuid`）を取得する。
  - 新世代番号: `newGeneration := currentGen + 1`。
  - トランザクションに入る前に、`generateNames` で新しい名前を `count` 個生成する。失敗した場合は DB に触れずエラーを返す。
  - 名前生成が成功したら、1 つのトランザクション内で：
    1. 旧ホストを削除: `DELETE FROM hosts WHERE world_id=$1::uuid`
    2. 新ホストを挿入: `makeCenter(names[i], seed, i, newGeneration)` で作成した各局を `insertCenter` する。
    3. コミットする。
  - 電話番号は `mix(seed, index, 0)` で算出されるため、世代が変わっても不変である。
- **FR-003（生成処理の非同期完走 / リクエスト切り離し）**:
  - `bootstrapWorld` および `resetHosts` は、生成処理（`GetOrCreate` / `ResetHosts`）に渡す context に、リクエストのキャンセルが伝播しないコンテキスト（`context.WithoutCancel(r.Context())` に 75 秒タイムアウトを付与したもの）を使用する。
  - クライアントが切断しても、生成処理と DB コミットは最後まで完走する。
  - アドバイザリロック（`pg_advisory_lock`）により、同一世界キーに対する同時のリクエストは直列化され、先行処理が完了した後は即座に DB から読み出される。

### Web 要件

- **FR-004（メインメニューの `1r` コマンド）**:
  - メインメニュー表示中、`1r` または `1R` の入力で生成ホストの再生成を開始する。
  - 実行時、端末画面に「生成ホスト情報を再生成しています...」を表示する（`showRegeneratingHosts()`）。
  - ESC キーでメインメニューに戻ることができる（バックグラウンドの再生成処理は継続する）。
  - 再生成が完了したら、`centersRef.current` を新しい生成局とプリセット局でマージし直し、局数表示を更新する。
  - 完了時、端末がまだ再生成画面を表示中であれば、センター・リスト（電話帳）を開く。
  - 再生成が失敗した場合、端末にエラーメッセージを表示し、ESC でメインメニューへ戻れるようにする。
- **FR-005（API エンドポイント関数）**:
  - `CenterDirectory.ts` に `worldResetHostsEndpoint(wsURL, pageURL, localPage)` および `resetWorldHosts(wsURL)` を追加する。
  - `POST` メソッドで `key` クエリパラメータを付与してリクエストし、`parseDirectory` でセンター一覧を返す。
- **FR-006（生成局取得の自動再試行）**:
  - 生成局（`/api/world/bootstrap`）の取得が失敗した場合、自動的に指数バックオフ（2秒、10秒、30秒、以後30秒）で再試行する。
  - ブラウザの `online` イベント（`window.addEventListener('online', ...)`）を受信した際にも即座に再試行する。
  - 成功したら再試行ループを終了し、ディレクトリを更新する。
- **FR-007（電話帳の失敗時注記）**:
  - 生成局の取得失敗中、電話帳（`TerminalCenterDirectory`）の notice に「ほかのセンターの読み込みに失敗しました（再試行中...）」を設定し、開いている一覧を再描画する。
  - 生成局の取得に成功したら、notice をクリア（空文字）して再描画する。
- **FR-008（プリセット局取得失敗時のエラー画面遷移）**:
  - 起動直後にユーザーが `1` を入力して待機中（`openDirectoryWhenReadyRef.current === true`）の状態で、`fetchDirectory` がエラーになった場合、`showDirectoryError()` を呼び出して直ちにエラー画面を描画する。

## 成功基準

- **SC-001**: `go -C apps/server test ./...`、`npm --prefix apps/web test`、`npm --prefix apps/web run build` がすべて通る。
- **SC-002**: `worldcatalog.Store` のテストで、`ResetHosts` 後も電話番号と ID が同一であり、`generation` が上がり、スペックや名前が更新されることが検証されている。
- **SC-003**: `resetHosts` ハンドラのテストで、POST メソッドの検証、不正なキーの検証、正常系の応答形式が検証されている。
- **SC-004**: クライアントが切断しても、生成コンテキストがキャンセルされないことがコード上で担保されている。
- **SC-005**: Web のテストで、`1r` 入力時の動作、再試行ロジック、notice の設定、プリセット局エラー時の画面遷移が検証されている。

## スコープ外（変更しない）

- テレホーダイ登録番号（`VITE_TELEHODAI_NUMBERS` の既定値 `0920000196`）およびターミナル・モード案内文の例示番号。
- 生成局に実際にダイヤルできるようにすること（ホスト実体の作成や `NO ANSWER` の挙動変更）。
- 局の定義（preset）の変更。
- 世界の削除（`ResetWorld`）を Web から呼べるようにすること。
- 生成局の件数（100 局）の変更。
