# LAmnda Python Deploy

- zips all files from the configuration in a file
- upload the zip file to S3 Bucket
- Update function code of the given Lambda function
- clean cloudwatch logs

## First run

The first run create a config file named `lapd.yml` in the current directory.

For now only the "default" function is used.

```yml
functions:
- name: default
  filter:
  - base_path: .
    relative_path: src
    include:
    - '*'
    exclude: []
  - base_path: .venv/lib/python3.11/site-packages/
    relative_path: .
    include:
    - '*'
    exclude: []
s3_bucket: lapd
package: deploy.zip
local_package_name: deploy.zip
```

Now you have to provide these informations
- `s3_bucket` S3 bucket name for the zip file
- `package` S3 key for the zip file
- `local_package_name` local zip file name
- `functions` list of functions to deploy
- `functions.name` function name
- `functions.include` list of files to include in the zip file
- `functions.include.base_path` base path for the files
- `functions.include.relative_path` relative path for the files
- `functions.include.filter` list of files to include

## Usage


```bash
lapd -function lambdafunctionname -purge
```

Output

```bash
2023/11/13 21:27:50 Adding files to zip
2023/11/13 21:27:50 Added:  {. src [*] []}
2023/11/13 21:27:54 Added:  {.venv/lib/python3.11/site-packages/ . [*] []}
2023/11/13 21:27:54 Upload
2023/11/13 21:28:05 Deploy code
2023/11/13 21:28:05 Deploying function lambdafunctionname
2023/11/13 21:28:07 Purge logs
```
