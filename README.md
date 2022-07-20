# geo-sqlite-dumper

Tool to convert sqlite files into KML / CSV files for plotting coordinates on a map.


## Usage:

```
$ ./geo-sqlite-dumper -h
geo-sqlite-dumper - Tool to view the contents of a sqlite file (github.com/pschou/geo-sqlite-dumper)
Apache 2.0 license, provided AS-IS -- not responsible for loss.
Usage implies agreement.  Version: 0.1.20220720.1213

Usage: ./geo-sqlite-dumper [options...] [files...]

Options:
      --debug    Verbose output
  -e, --event TIME  Event qualifier, time between events to split on  (Default: 2h0m0s)
  -q, --query SQL  Custom query for SQLite  (Default: "")
      --timeout TIME  Busy timeout for SQLite calls  (Default: 10s)
KML options:
      --kml FILENAME  Export to KML file  (Default: "")
  -N, --name TEXT  Name to use for base KML folder  (Default: "geo-sqlite-dumper")
CSV option:
      --csv FILENAME  Export to CSV file  (Default: "")
```

## Example

Export to kml file:
```
$ geo-sqlite-dumper --kml sample.kml sample.sqlite
```

Export to csv file:
```
$ geo-sqlite-dumper --csv sample.csv sample.sqlite
```

Export to kml and csv file:
```
$ geo-sqlite-dumper --kml sample.kml --csv sample.csv sample.sqlite
```

More than one file can be specified at one time like this (all the data will be placed in one output file):
```
$ geo-sqlite-dumper --kml sample.kml sample.sqlite another_file.sqlite
```
