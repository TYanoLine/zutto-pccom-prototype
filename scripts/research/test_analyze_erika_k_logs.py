import importlib.util
import tempfile
import unittest
import sys
from pathlib import Path

SCRIPT = Path(__file__).with_name("analyze_erika_k_logs.py")
spec = importlib.util.spec_from_file_location("analyzer", SCRIPT)
mod = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = mod
spec.loader.exec_module(mod)

class AnalyzerTest(unittest.TestCase):
    def test_extracts_protocol_without_message_bodies(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            p = root / "grassroots/erika-k/x/normalized/log.txt"
            p.parent.mkdir(parents=True)
            p.write_text("""ERIKA-K VER1.93\nInput Your ID: GUEST\n〖1〗boards ( BM ) 〖4〗call ( C )\nMAIN>M:MENU -> 4\n(BJ\\40) BOARD>M:MENU ?:HELP -> 90\n90-- 14 95/08/02 03:35 USER 8 private subject\n[Ret/番号]読む [A]ｱﾍﾟﾝﾄﾞ [W]書く [.]戻る [?]HELP\nAPPEND 6 1\n95/06/08 00:34:19 USER arbitrary private message follows\n""", encoding="utf-8")
            ev = mod.analyze_file(p, root)
            vals = {(e.kind, e.value) for e in ev}
            self.assertIn(("version", "ERIKA-K VER1.93"), vals)
            self.assertIn(("direct_command", "BM"), vals)
            self.assertIn(("direct_command", "C"), vals)
            self.assertIn(("append_event", "APPEND 6 1"), vals)
            self.assertTrue(any(k == "article_list" and "numeric_field_after_author=8" in v for k, v in vals))
            self.assertFalse(any("private subject" in e.value for e in ev))
            self.assertFalse(any("arbitrary private message" in e.value for e in ev))

if __name__ == "__main__":
    unittest.main()
