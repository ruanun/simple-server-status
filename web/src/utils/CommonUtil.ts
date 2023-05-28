
export function readableBytes(bytes: number) {
    if (!bytes) {
        return '0B'
    }
    var i = Math.floor(Math.log(bytes) / Math.log(1024)),
        sizes = ["B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"];
    return parseFloat((bytes / Math.pow(1024, i)).toFixed(2)) + sizes[i];
}