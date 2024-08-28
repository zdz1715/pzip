package org.demo;

import java.nio.file.Path;
import java.util.zip.ZipEntry;

public interface UnzipCallback {
    void extractAfter(ZipEntry ze, Path path);
}
