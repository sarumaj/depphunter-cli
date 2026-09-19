using System;
using System.Collections.Generic;
using static System.Math;
using Json = Newtonsoft.Json.Linq;
using MyApp.Core.Services;
using MyApp.Core.Missing;
using MyApp.Web.Controllers;
global using Serilog.Sinks.Console;
using Serilog;
using Microsoft.Extensions.Hosting;
using Microsoft.AspNetCore.Builder;
using Dapper;

namespace MyApp.Web;

public class Program
{
    public static void Main(string[] args) { }
}

public interface IService { void Run(); }
public record Person(string Name);
public struct Pt { }
public enum Mode { A }
public delegate void Handler();
