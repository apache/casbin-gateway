-- Copyright 2026 The casbin Authors. All Rights Reserved.
--
-- Licensed under the Apache License, Version 2.0 (the "License");
-- you may not use this file except in compliance with the License.
-- You may obtain a copy of the License at
--
--      http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing, software
-- distributed under the License is distributed on an "AS IS" BASIS,
-- WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
-- See the License for the specific language governing permissions and
-- limitations under the License.

use framework "Foundation"

on run
end run

on «event GURLGURL» linkText
	set linkString to current application's NSString's stringWithString:linkText
	set linkData to linkString's dataUsingEncoding:(current application's NSUTF8StringEncoding)
	if (linkData's |length|()) > 8192 then return

	set helperURL to current application's NSBundle's mainBundle()'s bundleURL()
	set contentsURL to helperURL's URLByDeletingLastPathComponent()'s URLByDeletingLastPathComponent()
	set launcherURL to contentsURL's URLByAppendingPathComponent:"MacOS/casbin-gateway-desktop"
	set inputPipe to current application's NSPipe's pipe()
	set launcher to current application's NSTask's new()
	launcher's setLaunchPath:(launcherURL's |path|())
	launcher's setArguments:{"open-link-stdin"}
	launcher's setStandardInput:inputPipe
	launcher's |launch|()
	set writer to inputPipe's fileHandleForWriting()
	writer's writeData:linkData
	writer's closeFile()
end «event GURLGURL»
