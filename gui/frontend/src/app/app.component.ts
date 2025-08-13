import { Component } from '@angular/core';
import { FetchFiles, OpenInputFileDialog, OpenOutputDirectoryDialog } from '../../wailsjs/go/main/App';
import { RunCLIFetch } from '../../wailsjs/go/main/App';


@Component({
  selector: 'app-root',
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.scss']
})
export class AppComponent {
  status = 'Ready';
  inputFilePath = '';
  outputDirPath = '';

  onSelectOutputDirectory() {
    OpenOutputDirectoryDialog().then((dirPath: string) => {
      if (dirPath) {
        this.outputDirPath = dirPath;
      }
    }).catch(err => {
      this.status = "Error: " + err;
    });
  }

  onFetchFiles() {
    if (!this.inputFilePath || !this.outputDirPath) {
      this.status = "Please select both an input TCIA file and an output directory.";
      return;
    }
    RunCLIFetch(this.inputFilePath, this.outputDirPath)
      .then((result: string) => {
        this.status = result;
      })
      .catch(err => {
        this.status = "Error: " + err;
      });
  }

  onSelectInputFile() {
    OpenInputFileDialog().then((filePath: string) => {
      if (filePath) {
        this.inputFilePath = filePath;
      }
    }).catch(err => {
      this.status = "Error: " + err;
    });
  }
}
