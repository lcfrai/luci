// Read-only native probes for sample-windows-owned.ps1. No process termination,
// command-line/environment reads, remote endpoints or Docker mutations.
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.IO.Pipes;
using System.Runtime.InteropServices;
using System.Text;
using System.Text.RegularExpressions;

public static class LuciWindowsResourceNative {
    [StructLayout(LayoutKind.Sequential)] struct FileTime { public uint Low, High; }
    [StructLayout(LayoutKind.Sequential)] struct MemoryStatus {
        public uint Length, Load;
        public ulong TotalPhysical, AvailablePhysical, CommitLimit, AvailableCommit, TotalVirtual, AvailableVirtual, ExtendedVirtual;
    }
    [StructLayout(LayoutKind.Sequential)] struct MemoryCounters {
        public uint Size, PageFaults;
        public UIntPtr PeakWorkingSet, WorkingSet, PeakPagedPool, PagedPool, PeakNonPagedPool, NonPagedPool, Pagefile, PeakPagefile, PrivateUsage;
    }
    [StructLayout(LayoutKind.Sequential)] struct IOCounters {
        public ulong ReadOperations, WriteOperations, OtherOperations, ReadBytes, WriteBytes, OtherBytes;
    }
    [StructLayout(LayoutKind.Sequential, CharSet=CharSet.Unicode)] struct ProcessEntry {
        public uint Size, Usage, ProcessID;
        public UIntPtr Heap;
        public uint ModuleID, Threads, ParentID;
        public int Priority;
        public uint Flags;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst=260)] public string Executable;
    }
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool GlobalMemoryStatusEx(ref MemoryStatus value);
    [DllImport("kernel32.dll")] static extern uint GetActiveProcessorCount(ushort group);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool GetSystemTimes(out FileTime idle, out FileTime kernel, out FileTime user);
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)] static extern bool GetDiskFreeSpaceEx(string path, out ulong available, out ulong total, out ulong free);
    [DllImport("kernel32.dll", SetLastError=true)] static extern IntPtr OpenProcess(uint access, bool inherit, uint pid);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool GetProcessTimes(IntPtr handle, out FileTime creation, out FileTime exit, out FileTime kernel, out FileTime user);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool K32GetProcessMemoryInfo(IntPtr handle, ref MemoryCounters counters, uint size);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool GetProcessIoCounters(IntPtr handle, out IOCounters counters);
    [DllImport("kernel32.dll", SetLastError=true)] static extern uint WaitForSingleObject(IntPtr handle, uint milliseconds);
    [DllImport("kernel32.dll", SetLastError=true)] static extern bool CloseHandle(IntPtr handle);
    [DllImport("kernel32.dll", SetLastError=true)] static extern IntPtr CreateToolhelp32Snapshot(uint flags, uint process);
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)] static extern bool Process32FirstW(IntPtr snapshot, ref ProcessEntry entry);
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)] static extern bool Process32NextW(IntPtr snapshot, ref ProcessEntry entry);
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)] static extern bool CreateDirectoryW(string path, IntPtr attributes);
    static ulong Ticks(FileTime value) { return ((ulong)value.High << 32) | value.Low; }
    public sealed class Host {
        public ulong? PhysicalTotalBytes, PhysicalAvailableBytes, CommitLimitBytes, CommitAvailableBytes, CPUTotalTicks, CPUIdleTicks;
        public uint LogicalProcessors;
    }
    public sealed class Disk { public ulong TotalBytes, AvailableBytes; }
    public sealed class ProcessRow {
        public uint ProcessID;
        public ulong CreationFileTime, CPU100ns;
        public bool Alive;
        public ulong? WorkingSetBytes, PrivateCommitBytes, ReadBytes, WriteBytes, OtherBytes, ReadOperations, WriteOperations;
    }
    public sealed class ParentRow { public uint ProcessID, ParentID; }
    public sealed class Topology { public List<ParentRow> Rows = new List<ParentRow>(); public bool Complete; }
    public static Host ReadHost() {
        Host result = new Host(); MemoryStatus memory = new MemoryStatus(); memory.Length=(uint)Marshal.SizeOf(memory);
        if (GlobalMemoryStatusEx(ref memory)) {
            result.PhysicalTotalBytes=memory.TotalPhysical; result.PhysicalAvailableBytes=memory.AvailablePhysical;
            result.CommitLimitBytes=memory.CommitLimit; result.CommitAvailableBytes=memory.AvailableCommit;
        }
        result.LogicalProcessors=GetActiveProcessorCount(0xffff);
        FileTime idle,kernel,user;
        if (GetSystemTimes(out idle,out kernel,out user)) { result.CPUTotalTicks=Ticks(kernel)+Ticks(user); result.CPUIdleTicks=Ticks(idle); }
        return result;
    }
    public static Disk ReadDisk(string path) {
        ulong available,total,free;
        if (!GetDiskFreeSpaceEx(path,out available,out total,out free)) return null;
        return new Disk {TotalBytes=total,AvailableBytes=available};
    }
    public static ProcessRow ReadProcess(uint pid) {
        // Handles pin a particular process object even if its numeric PID is reused.
        IntPtr handle=OpenProcess(0x1000|0x10|0x100000,false,pid);
        if (handle==IntPtr.Zero) handle=OpenProcess(0x1000|0x100000,false,pid);
        if (handle==IntPtr.Zero) return null;
        try {
            FileTime creation,exit,kernel,user;
            if (!GetProcessTimes(handle,out creation,out exit,out kernel,out user)) return null;
            ProcessRow result=new ProcessRow {ProcessID=pid,CreationFileTime=Ticks(creation),CPU100ns=Ticks(kernel)+Ticks(user),Alive=WaitForSingleObject(handle,0)==258};
            MemoryCounters memory=new MemoryCounters(); memory.Size=(uint)Marshal.SizeOf(memory);
            if (K32GetProcessMemoryInfo(handle,ref memory,memory.Size)) { result.WorkingSetBytes=memory.WorkingSet.ToUInt64(); result.PrivateCommitBytes=memory.PrivateUsage.ToUInt64(); }
            IOCounters io;
            if (GetProcessIoCounters(handle,out io)) { result.ReadBytes=io.ReadBytes;result.WriteBytes=io.WriteBytes;result.OtherBytes=io.OtherBytes;result.ReadOperations=io.ReadOperations;result.WriteOperations=io.WriteOperations; }
            return result;
        } finally { CloseHandle(handle); }
    }
    public static Topology ReadTopology() {
        Topology result=new Topology(); IntPtr handle=CreateToolhelp32Snapshot(2,0);
        if (handle==IntPtr.Zero || handle==new IntPtr(-1)) return result;
        try {
            ProcessEntry entry=new ProcessEntry();entry.Size=(uint)Marshal.SizeOf(entry);
            if (!Process32FirstW(handle,ref entry)) return result;
            do {
                if (result.Rows.Count>=32768) return result;
                result.Rows.Add(new ParentRow {ProcessID=entry.ProcessID,ParentID=entry.ParentID});
            } while (Process32NextW(handle,ref entry));
            result.Complete=Marshal.GetLastWin32Error()==18;
            return result;
        } finally { CloseHandle(handle); }
    }
    public static void CreateFreshDirectory(string path) {
        // Atomic failure if it already exists; never tighten ACLs on an existing folder.
        if (!CreateDirectoryW(path,IntPtr.Zero)) throw new IOException("Fresh output directory could not be created");
    }
    static int Remaining(Stopwatch watch,int budget) {
        int value=budget-(int)watch.ElapsedMilliseconds;
        if (value<=0) throw new TimeoutException("Docker observation deadline");return value;
    }
    public static string DockerGet(string pipeName,string path,int timeoutMs) {
        if (!Regex.IsMatch(pipeName,@"\A[A-Za-z0-9_.-]{1,80}\z") ||
            !Regex.IsMatch(path,@"\A/(info|containers/json\?all=1|containers/[0-9a-f]{64}/stats\?stream=false&one-shot=true)\z") || timeoutMs<1 || timeoutMs>5000)
            throw new ArgumentException("Invalid bounded local Docker observation");
        const int limit=2*1024*1024; Stopwatch watch=Stopwatch.StartNew();
        using (NamedPipeClientStream pipe=new NamedPipeClientStream(".",pipeName,PipeDirection.InOut,PipeOptions.Asynchronous)) {
            pipe.Connect(Remaining(watch,timeoutMs));
            byte[] request=Encoding.ASCII.GetBytes("GET "+path+" HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\nAccept: application/json\r\n\r\n");
            var write=pipe.WriteAsync(request,0,request.Length);
            if (!write.Wait(Remaining(watch,timeoutMs))) throw new TimeoutException("Docker observation write deadline");
            using (MemoryStream response=new MemoryStream()) {
                byte[] block=new byte[8192];
                while (true) {
                    var read=pipe.ReadAsync(block,0,block.Length);
                    if (!read.Wait(Remaining(watch,timeoutMs))) throw new TimeoutException("Docker observation read deadline");
                    int count=read.Result;if(count==0)break;
                    if(response.Length+count>limit)throw new IOException("Docker observation body bound");response.Write(block,0,count);
                }
                return DecodeResponse(response.ToArray());
            }
        }
    }
    public static string DecodeResponse(byte[] bytes) {
        string text=Encoding.UTF8.GetString(bytes);int boundary=text.IndexOf("\r\n\r\n",StringComparison.Ordinal);
        if(boundary<0||boundary>32768||bytes.Length>2*1024*1024)throw new IOException("Docker observation headers");
        string headers=text.Substring(0,boundary);
        if(!Regex.IsMatch(headers,@"\AHTTP/1\.[01] 200(?: |\r\n)"))throw new IOException("Docker observation HTTP status");
        // JSON returned by the daemon is UTF-8. Chunk lengths count bytes, not characters.
        int byteBoundary=Encoding.UTF8.GetByteCount(text.Substring(0,boundary+4));
        if(!Regex.IsMatch(headers,@"(?im)^Transfer-Encoding:\s*chunked\s*$"))return Encoding.UTF8.GetString(bytes,byteBoundary,bytes.Length-byteBoundary);
        using(MemoryStream body=new MemoryStream()) {
            int offset=byteBoundary;
            while(true) {
                int end=offset;while(end+1<bytes.Length&&(bytes[end]!=13||bytes[end+1]!=10))end++;
                if(end+1>=bytes.Length||end-offset>128)throw new IOException("Docker observation chunk framing");
                string line=Encoding.ASCII.GetString(bytes,offset,end-offset).Split(';')[0];int size;
                if(!int.TryParse(line,System.Globalization.NumberStyles.HexNumber,System.Globalization.CultureInfo.InvariantCulture,out size)||size<0)throw new IOException("Docker observation chunk size");
                offset=end+2;if(size==0)break;
                if(size>bytes.Length-offset-2||bytes[offset+size]!=13||bytes[offset+size+1]!=10)throw new IOException("Docker observation truncated chunk");
                body.Write(bytes,offset,size);offset+=size+2;
            }
            return Encoding.UTF8.GetString(body.ToArray());
        }
    }
}
