import unittest
import archive
import pull_metadata


class PullMetadataTests(unittest.TestCase):
    def test_matches_go_sorted_utf8_html_escaped_without_newline(self):
        self.assertEqual(pull_metadata.encoded({'z': '\u2028', 'a': 'é<&>'}),
                         b'{"a":"\xc3\xa9\\u003c\\u0026\\u003e","z":"\\u2028"}')

    def test_nonfinite_values_cannot_enter_receipt(self):
        with self.assertRaises(ValueError): pull_metadata.encoded({'size': float('nan')})

    def test_noncanonical_scalar_types_refuse(self):
        for value in (None, 1.0, 1 << 53, {2: 'key'}, ['ok', None]):
            with self.subTest(value=value), self.assertRaises(ValueError):
                pull_metadata.encoded(value)

    def test_v1_bytes_keep_original_newline_and_ascii_escaping(self):
        self.assertEqual(archive.encoded({'z': '\u2028', 'a': 'é<&>'}),
                         b'{"a":"\\u00e9<&>","z":"\\u2028"}\n')


if __name__ == '__main__': unittest.main()
