# geo-sqlite-dumper

Tool to convert geo sqlite files into KML / CSV files for plotting coordinates on a map.


## Usage:

```
$ ./geo-sqlite-dumper -h
geo-sqlite-dumper - Tool to view the contents of a geo sqlite file (github.com/pschou/geo-sqlite-dumper)
Apache 2.0 license, provided AS-IS -- not responsible for loss.
Usage implies agreement.  Version: 0.1.20220907.1055

Usage: ../geo-sqlite-dumper [options...] [files...]

Options:
      --debug       Verbose output
  -e, --event-time TIME  Event qualifier, time between events to split on  (Default: 2h0m0s)
      --force       Ignore file/read errors and continue building output
      --list FILE   File with list of files to process, one line per file  (Default: "")
  -q, --query SQL   Custom query for SQLite  (Default: "")
  -E, --show-event-lines  Show event lines for a series of points within event-time
      --timeout TIME  Busy timeout for SQLite calls  (Default: 10s)
KML options:
      --kml FILENAME  Export to KML file  (Default: "")
  -N, --name TEXT   Name to use for base KML folder  (Default: "geo-sqlite-dumper")
CSV options:
      --csv FILENAME  Export to CSV file  (Default: "")
      --delimiter DELIM  Delimiter for CSV output  (Default: ",")
      --escape-ascii T/F  Escape non-ascii characters, useful for using tools that are not
                    ascii safe/sanitizing special characters  (Default: false)
XLSX options:
      --sheet NAME  Sheet name to use in export  (Default: "geo-sqlite-dumper")
      --xlsx_file FILENAME  Export to XLSX file  (Default: "")
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
