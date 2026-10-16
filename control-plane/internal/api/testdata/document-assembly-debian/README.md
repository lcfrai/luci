These exact source-metadata fixtures exercise the private policy normalizer.
They contain no observed key/signature verification or accepted assembly closure.

- `official-policy-13bd77f4.json`: retained official policy v3, 108143 bytes,
  SHA256 `13bd77f40356ef37ae01f3baf05172a035423815b33e9a292a7736ec699150a5`.
- `producer-config-64583d9f.json`: retained LOCAL117 producer v4 configuration,
  41817 bytes, SHA256
  `64583d9f02277a0f311d1f6776850471b1720f18ea22f24bb93381c6c00b5205`.

The local `.gitattributes` preserves exact metadata bytes. Paths and URLs inside
these records are source attribution only; the tests neither open them nor fetch
objects, execute verifiers, import keys, run packages or access a Docker engine.
