# Reset safety boundary

The browser world key identifies a world but is not an authorization credential. Therefore destructive reset operations must never authorize solely from `worldKey`; they additionally require the server-only debug reset token.
