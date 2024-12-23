cd dashboard && goreleaser release --snapshot --clean
cd ../
cd agent && goreleaser release --snapshot --clean