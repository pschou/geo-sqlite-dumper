# sqlite2kml

Tool to convert sqlite files into KML files for plotting coordinates on a map.


## Usage:

```
$ ./sqlite2kml -h
SqlLite3 to KML (github.com/pschou/sqlite2kml)
Apache 2.0 license, provided AS-IS -- not responsible for loss.
Usage implies agreement.  Version: 0.1.20220718.1028

Usage: ./sqlite2kml [options...] [files...]

Options:
  --debug            Verbose output
  --event TIME       Event qualifier, time between events to split on  (Default: 10m0s)
  --name TEXT        Name to use for base KML folder  (Default: "sqlite2kml")
  --open-file TRUE/FALSE  Open all the file-named-folders when KML is loaded  (Default: false)
```

## Example

```
$ sqlite2kml sample.sqlite > sample.kml
```

Note that more than one file can be specified at one time, like this:

```
$ sqlite2kml sample.sqlite data.sqlite > map.kml
```
