# Reference data

`voc.csv` comes from `python-wrapper/tests/data/voc.csv` in Sensirion's
`gas-index-algorithm` release `3.2.2`, commit
`12188f5618ca7c68d22252ea7e61bb54830cd168`.

SHA-256: `a237a1284544ab7123442518ad40c42c8e0c506f809b28910e63b5c3b948a795`

Each row contains an SGP40 raw signal and the expected VOC Index. The upstream
test permits an absolute difference of 1, which this repository preserves for
cross-language floating-point compatibility.
