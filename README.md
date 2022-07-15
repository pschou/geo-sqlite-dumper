# sqlite2kml

Tool to convert sqlite files into KML files for plotting coordinates on a map.


## Usage:

```
$ ./sqlite2kml -h
SqlLite3 to KML (github.com/pschou/sqlite2kml)
Apache 2.0 license, provided AS-IS -- not responsible for loss.
Usage implies agreement.  Version: 0.1.20220715.1435

Usage: ./sqlite2kml [options...]

Option:
  --debug  Verbose output
```

## Example

```
$ sqlite2kml sample.sqlite > sample.kml
```

Note that more than one file can be specified at one time, like this:

```
$ sqlite2kml sample.sqlite data.sqlite > map.kml
```
